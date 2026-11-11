// KTEX 解码（只取第一层 mip），支持 DXT1 / DXT3 / DXT5 / RGBA，输出 RGBA，行顺序与文件中一致
const PIXEL_DXT1 = 0
const PIXEL_DXT3 = 1
const PIXEL_DXT5 = 2
const PIXEL_RGBA = 4

function decodeKTEX(buf) {
  if (buf.toString('ascii', 0, 4) !== 'KTEX') throw new Error('不是 KTEX 文件')
  const header = buf.readUInt32LE(4)
  const pixelFormat = (header >>> 4) & 0x1f
  const mipCount = (header >>> 13) & 0x1f
  const width = buf.readUInt16LE(8)
  const height = buf.readUInt16LE(10)
  const dataStart = 8 + mipCount * 10
  const src = buf.subarray(dataStart)
  const data = Buffer.alloc(width * height * 4)

  if (pixelFormat === PIXEL_RGBA) {
    src.copy(data, 0, 0, data.length)
    return { width, height, data }
  }
  const blockSize = pixelFormat === PIXEL_DXT1 ? 8 : 16
  const bw = Math.ceil(width / 4)
  const bh = Math.ceil(height / 4)
  for (let by = 0; by < bh; by++) {
    for (let bx = 0; bx < bw; bx++) {
      const off = (by * bw + bx) * blockSize
      const pixels = decodeBlock(src, off, pixelFormat)
      for (let py = 0; py < 4; py++) {
        for (let px = 0; px < 4; px++) {
          const x = bx * 4 + px
          const y = by * 4 + py
          if (x >= width || y >= height) continue
          const i = (y * width + x) * 4
          const p = (py * 4 + px) * 4
          data[i] = pixels[p]
          data[i + 1] = pixels[p + 1]
          data[i + 2] = pixels[p + 2]
          data[i + 3] = pixels[p + 3]
        }
      }
    }
  }
  return { width, height, data }
}

function decodeBlock(src, off, format) {
  const out = new Uint8Array(64)
  let colorOff = off
  let alpha
  if (format === PIXEL_DXT5) {
    alpha = dxt5Alpha(src, off)
    colorOff = off + 8
  } else if (format === PIXEL_DXT3) {
    alpha = new Uint8Array(16)
    for (let i = 0; i < 16; i++) {
      const byte = src[off + (i >> 1)]
      alpha[i] = ((i & 1 ? byte >> 4 : byte & 0xf) * 17) & 0xff
    }
    colorOff = off + 8
  }

  const c0 = src.readUInt16LE(colorOff)
  const c1 = src.readUInt16LE(colorOff + 2)
  const bits = src.readUInt32LE(colorOff + 4)
  const colors = [rgb565(c0), rgb565(c1)]
  if (c0 > c1 || format !== PIXEL_DXT1) {
    colors.push(mix(colors[0], colors[1], 2, 1, 3), mix(colors[0], colors[1], 1, 2, 3))
  } else {
    colors.push(mix(colors[0], colors[1], 1, 1, 2), [0, 0, 0, 0])
  }
  for (let i = 0; i < 16; i++) {
    const c = colors[(bits >>> (i * 2)) & 3]
    out[i * 4] = c[0]
    out[i * 4 + 1] = c[1]
    out[i * 4 + 2] = c[2]
    out[i * 4 + 3] = alpha ? alpha[i] : c[3]
  }
  return out
}

function dxt5Alpha(src, off) {
  const a0 = src[off]
  const a1 = src[off + 1]
  const table = [a0, a1]
  if (a0 > a1) for (let i = 1; i < 7; i++) table.push(Math.round(((7 - i) * a0 + i * a1) / 7))
  else {
    for (let i = 1; i < 5; i++) table.push(Math.round(((5 - i) * a0 + i * a1) / 5))
    table.push(0, 255)
  }
  // 6 字节共 48 位，每个像素 3 位
  let bits = 0n
  for (let i = 0; i < 6; i++) bits |= BigInt(src[off + 2 + i]) << BigInt(8 * i)
  const alpha = new Uint8Array(16)
  for (let i = 0; i < 16; i++) alpha[i] = table[Number((bits >> BigInt(3 * i)) & 7n)]
  return alpha
}

function rgb565(c) {
  const r = (c >> 11) & 0x1f
  const g = (c >> 5) & 0x3f
  const b = c & 0x1f
  return [(r << 3) | (r >> 2), (g << 2) | (g >> 4), (b << 3) | (b >> 2), 255]
}

function mix(a, b, wa, wb, d) {
  return [0, 1, 2].map((k) => Math.round((a[k] * wa + b[k] * wb) / d)).concat(255)
}

module.exports = { decodeKTEX }
