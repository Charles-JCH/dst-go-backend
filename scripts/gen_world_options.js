// 从游戏的 customize.lua + chinese_s.po + 默认存档的 leveldataoverride.lua 生成世界设置目录和模板，
// 并从游戏图集中切出每个设置项的图标。游戏更新增加了新设置项时重新生成：
//   world_options.json → 后端 internal/service/dst/
//   leveldataoverride_*.lua.tmpl → 后端 internal/service/dst/templates/
//   world-icons/<设置项>.webp → 前端 public/world-icons/
//
// 准备：
//   在已部署游戏的服务器上取出两个文件
//     unzip -p ~/dst/data/databundles/scripts.zip scripts/map/customize.lua > customize.lua
//     unzip -p ~/dst/data/databundles/scripts.zip scripts/languages/chinese_s.po > chinese_s.po
//   专用服务器不带界面贴图，从 Steam 客户端的 data/images/ 取三套图集（.tex + .xml）：
//     customisation、worldgen_customization、worldsettings_customization
// 用法（需要 Lua 虚拟机 fengari 和图片库 sharp：在任意临时目录 npm install fengari@0.1.4 sharp@0.33.5，
// 再用 NODE_PATH 指向该目录的 node_modules）：
//   node gen_world_options.js <customize.lua> <chinese_s.po> <默认存档目录> <图集目录> <输出目录>
const fs = require('fs')
const path = require('path')
const { lua, lauxlib, lualib, to_luastring, to_jsstring } = require('fengari')
const sharp = require('sharp')
const { decodeKTEX } = require('./ktex')

const [customizePath, poPath, archiveDir, atlasDir, outDir] = process.argv.slice(2)
/** 图标输出尺寸，页面按一半显示，兼顾高分屏 */
const ICON_SIZE = 64

// 1. 执行 customize.lua：STRINGS.xxx 换成字符串常量，其余依赖用桩代替
let src = fs.readFileSync(customizePath, 'utf8')
src = src.replace(/STRINGS(\.[A-Za-z0-9_]+)+/g, (m) => `("${m}")`)

const prelude = `
LEVELCATEGORY = { WORLDGEN = "worldgen", SETTINGS = "settings" }
SPECIAL_EVENTS = { NONE = "none" }
function IsNotConsole() return true end
function IsPS4() return false end
function deepcopy(t) if type(t) ~= "table" then return t end local r = {} for k, v in pairs(t) do r[k] = deepcopy(v) end return r end
function shallowcopy(t) local r = {} for k, v in pairs(t) do r[k] = v end return r end
function GetTableSize(t) local n = 0 for _ in pairs(t) do n = n + 1 end return n end
function IsTableEmpty(t) return next(t) == nil end
function FunctionOrValue(v, ...) if type(v) == "function" then return v(...) end return v end
function stringidsorter(a, b) return a < b end
function table.contains(t, v) for _, x in ipairs(t) do if x == v then return true end end return false end
local stubs = {
  ["map/tasksets"] = { GetGenTaskLists = function() return nil end },
  ["map/startlocations"] = { GetGenStartLocations = function() return nil end },
  ["map/tasks"] = {},
  ["map/levels"] = { GetDataForLocation = function() return nil end },
  ["worldsettings_overrides"] = { Pre = setmetatable({}, { __index = function() return true end }), Post = {} },
}
function require(name) return stubs[name] end

local function enc(v)
  local t = type(v)
  if t == "string" then return string.format("%q", v):gsub("\\\\\\n", "\\\\n") end
  if t == "number" or t == "boolean" then return tostring(v) end
  if t ~= "table" then return "null" end
  if #v > 0 then
    local parts = {}
    for _, x in ipairs(v) do parts[#parts + 1] = enc(x) end
    return "[" .. table.concat(parts, ",") .. "]"
  end
  local parts = {}
  for k, x in pairs(v) do
    if type(x) ~= "function" and type(x) ~= "table" or (type(x) == "table") then
      parts[#parts + 1] = string.format("%q", tostring(k)) .. ":" .. enc(x)
    end
  end
  return "{" .. table.concat(parts, ",") .. "}"
end

local M = load(CUSTOMIZE_SRC)()
local out = {}
for _, shard in ipairs({ { "forest", true }, { "cave", false } }) do
  local loc, master = shard[1], shard[2]
  local list = {}
  for _, cat in ipairs({ { "worldgen", M.GetWorldGenOptions }, { "settings", M.GetWorldSettingsOptions } }) do
    for _, o in ipairs(cat[2](loc, master)) do
      if type(o.options) == "table" then
        local opts = {}
        for _, d in ipairs(o.options) do opts[#opts + 1] = { text = d.text, data = d.data } end
        list[#list + 1] = { name = o.name, category = cat[1], group = o.group, grouplabel = o.grouplabel, default = o.default, options = opts, atlas = o.atlas, image = o.image }
      end
    end
  end
  out[loc] = list
end
RESULT = enc(out)
`

const L = lauxlib.luaL_newstate()
lualib.luaL_openlibs(L)
lua.lua_pushstring(L, to_luastring(src))
lua.lua_setglobal(L, to_luastring('CUSTOMIZE_SRC'))
if (lauxlib.luaL_dostring(L, to_luastring(prelude)) !== 0) {
  console.error('Lua 执行失败:', to_jsstring(lua.lua_tostring(L, -1)))
  process.exit(1)
}
lua.lua_getglobal(L, to_luastring('RESULT'))
const raw = JSON.parse(to_jsstring(lua.lua_tostring(L, -1)))

// 2. 中文翻译：msgctxt "STRINGS.X.Y" → msgstr
const po = fs.readFileSync(poPath, 'utf8')
const tr = new Map()
const re = /msgctxt "([^"]+)"\r?\nmsgid "((?:[^"\\]|\\.)*)"\r?\nmsgstr "((?:[^"\\]|\\.)*)"/g
for (let m; (m = re.exec(po)); ) tr.set(m[1], (m[3] || m[2]).replace(/\\n/g, ' ').replace(/\\"/g, '"'))
const missing = new Set()
const translate = (text) =>
  text.replace(/STRINGS(\.[A-Za-z0-9_]+)+/g, (k) => {
    if (tr.has(k)) return tr.get(k)
    missing.add(k)
    return k.split('.').pop()
  }).replace(/ /g, '')

// 3. 默认存档的 overrides：key=字面量
function readArchive(file) {
  const text = fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n')
  const start = text.indexOf('  overrides={\n')
  const end = text.indexOf('\n  },\n', start)
  const values = {}
  for (const line of text.slice(start + '  overrides={\n'.length, end).split('\n')) {
    const m = line.match(/^\s*([a-z0-9_]+)=(.*?),?\s*$/)
    if (m) values[m[1]] = m[2]
  }
  return { header: text.slice(0, start), footer: text.slice(end + 1), values }
}

const warnings = []
const catalog = {}
for (const [loc, shard] of [['forest', 'Master'], ['cave', 'Caves']]) {
  const archive = readArchive(path.join(archiveDir, shard, 'leveldataoverride.lua'))
  const options = raw[loc].map((o) => {
    const values = o.options.map((d) => d.data)
    let def = o.default
    const archived = archive.values[o.name]
    if (archived !== undefined) {
      const v = archived.replace(/^"(.*)"$/, '$1')
      if (values.includes(v)) def = v
      else warnings.push(`${shard}.${o.name}: 默认存档的值 ${archived} 不在可选值中，改用游戏默认 ${o.default}`)
    }
    if (!values.includes(def)) warnings.push(`${shard}.${o.name}: 默认值 ${def} 不在可选值中`)
    return {
      name: o.name,
      label: translate(`STRINGS.UI.CUSTOMIZATIONSCREEN.${o.name.toUpperCase()}`),
      category: o.category,
      group: o.group,
      groupLabel: translate(o.grouplabel),
      default: def,
      options: o.options.map((d) => ({ value: d.data, label: translate(d.text) }))
    }
  })
  // 目录之外的键（如 has_ocean、layout_mode）原样保留默认存档的字面量
  const names = new Set(options.map((o) => o.name))
  const fixed = Object.fromEntries(Object.entries(archive.values).filter(([k]) => !names.has(k)))
  catalog[shard.toLowerCase()] = { options, fixed }

  const tmpl = archive.header + '  overrides={\n{{- range $i, $o := .}}{{if $i}},{{end}}\n    {{$o.Key}}={{$o.Value}}{{end}}\n' + archive.footer
  fs.writeFileSync(path.join(outDir, `leveldataoverride_${shard.toLowerCase()}.lua.tmpl`), tmpl)
}

fs.writeFileSync(path.join(outDir, 'world_options.json'), JSON.stringify(catalog, null, 1))
console.log('master', catalog.master.options.length, 'options,', Object.keys(catalog.master.fixed).length, 'fixed')
console.log('caves', catalog.caves.options.length, 'options,', Object.keys(catalog.caves.fixed).length, 'fixed')

// 4. 图标：按 xml 中的 uv 从图集切出，文件名为设置项名称（地面、洞穴同名设置项的图标相同）
const icons = new Map()
for (const o of [...raw.forest, ...raw.cave]) {
  if (!o.atlas || !o.image) warnings.push(`${o.name}: 没有图标`)
  else icons.set(o.name, { atlas: path.basename(o.atlas, '.xml'), image: o.image })
}

writeIcons().then(() => {
  console.log('icons', icons.size)
  console.log('missing translations:', [...missing].slice(0, 30))
  console.log('warnings:\n' + warnings.join('\n'))
})

async function writeIcons() {
  const iconDir = path.join(outDir, 'world-icons')
  fs.mkdirSync(iconDir, { recursive: true })
  const atlases = new Map()
  for (const [name, { atlas, image }] of icons) {
    if (!atlases.has(atlas)) atlases.set(atlas, loadAtlas(atlas))
    const { texture, elements } = atlases.get(atlas)
    const uv = elements[image]
    if (!uv) {
      warnings.push(`${name}: 图集 ${atlas} 中没有 ${image}`)
      continue
    }
    // 纹理按行倒置存储，v 从图片底部算起：在原始数据中直接按 v 取区域，再上下翻转
    const left = Math.round(uv.u1 * texture.width)
    const top = Math.round(uv.v1 * texture.height)
    const width = Math.round(uv.u2 * texture.width) - left
    const height = Math.round(uv.v2 * texture.height) - top
    const region = await sharp(texture.data, { raw: { width: texture.width, height: texture.height, channels: 4 } })
      .extract({ left, top, width, height })
      .raw()
      .toBuffer()
    await sharp(region, { raw: { width, height, channels: 4 } })
      .flip()
      .resize(ICON_SIZE, ICON_SIZE)
      .webp({ quality: 85 })
      .toFile(path.join(iconDir, `${name}.webp`))
  }
}

function loadAtlas(name) {
  const xml = fs.readFileSync(path.join(atlasDir, `${name}.xml`), 'utf8')
  const elements = {}
  const pattern = /<Element name="([^"]+)" u1="([\d.]+)" u2="([\d.]+)" v1="([\d.]+)" v2="([\d.]+)"/g
  for (const m of xml.matchAll(pattern)) elements[m[1]] = { u1: +m[2], u2: +m[3], v1: +m[4], v2: +m[5] }
  return { texture: decodeKTEX(fs.readFileSync(path.join(atlasDir, `${name}.tex`))), elements }
}
