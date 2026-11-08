package server

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

var ErrServerHasDst = errors.New("服务器仍有关联存档")

type Repository interface {
	Create(ctx context.Context, server *entity.Server) error
	GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error)
	List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error)
	Update(ctx context.Context, userID uint64, serverID uint64, fields UpdateFields) error
	Delete(ctx context.Context, userID uint64, serverID uint64) error

	// 以下为系统内部使用，不带 user_id 条件
	FindByID(ctx context.Context, serverID uint64) (*entity.Server, error)
	AgentTokenVersion(ctx context.Context, serverID uint64) (uint64, bool, error)
	SetSSHHostKey(ctx context.Context, serverID uint64, hostKey string) error
	IncrAgentTokenVersion(ctx context.Context, serverID uint64) (uint64, error)
	UpdateSysInfo(ctx context.Context, serverID uint64, fields SysInfoFields) error
}

type UpdateFields struct {
	Name       string
	Password   *string
	SSHHostKey *string
}

// SysInfoFields Agent 上报的系统配置，每次全部覆盖
type SysInfoFields struct {
	CPU    int
	RAM    int64
	Disk   int64
	OSName string
	Arch   string
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, server *entity.Server) error {
	return repo.WrapError(r.getDB(ctx).Create(server).Error)
}

func (r *repository) GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error) {
	return repo.First[entity.Server](r.getDB(ctx), "id = ? AND user_id = ?", serverID, userID)
}

func (r *repository) List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error) {
	db := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("user_id = ?", userID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var servers []entity.Server
	err := db.
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&servers).Error
	return servers, total, err
}

func (r *repository) Update(ctx context.Context, userID uint64, serverID uint64, fields UpdateFields) error {
	updates := map[string]any{"name": fields.Name}
	if fields.Password != nil {
		updates["password"] = *fields.Password
	}
	if fields.SSHHostKey != nil {
		updates["ssh_host_key"] = *fields.SSHHostKey
	}
	result := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("id = ? AND user_id = ?", serverID, userID).
		Updates(updates)
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	server, err := r.GetByID(ctx, userID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return repo.ErrNotFound
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, userID uint64, serverID uint64) error {
	db := r.getDB(ctx)
	dsts := db.Model(&entity.Dst{}).
		Select("1").
		Where("server_id = ?", serverID)
	result := db.
		Where("id = ? AND user_id = ?", serverID, userID).
		Where("NOT EXISTS (?)", dsts).
		Delete(&entity.Server{})
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	server, err := r.GetByID(ctx, userID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return repo.ErrNotFound
	}
	return ErrServerHasDst
}

func (r *repository) FindByID(ctx context.Context, serverID uint64) (*entity.Server, error) {
	return repo.First[entity.Server](r.getDB(ctx), "id = ?", serverID)
}

func (r *repository) AgentTokenVersion(ctx context.Context, serverID uint64) (uint64, bool, error) {
	server, err := repo.First[entity.Server](r.getDB(ctx).Select("agent_token_version"), "id = ?", serverID)
	if err != nil || server == nil {
		return 0, false, err
	}
	return server.AgentTokenVersion, true, nil
}

// SetSSHHostKey 只在尚未记录主机公钥时写入，首次信任后不会被覆盖
func (r *repository) SetSSHHostKey(ctx context.Context, serverID uint64, hostKey string) error {
	return repo.WrapError(r.getDB(ctx).
		Model(&entity.Server{}).
		Where("id = ? AND (ssh_host_key IS NULL OR ssh_host_key = '')", serverID).
		Update("ssh_host_key", hostKey).Error)
}

// IncrAgentTokenVersion 递增 Agent 令牌版本让旧令牌失效，返回新版本号
func (r *repository) IncrAgentTokenVersion(ctx context.Context, serverID uint64) (uint64, error) {
	result := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("id = ?", serverID).
		Update("agent_token_version", gorm.Expr("agent_token_version + 1"))
	if result.Error != nil {
		return 0, repo.WrapError(result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, repo.ErrNotFound
	}
	version, isFound, err := r.AgentTokenVersion(ctx, serverID)
	if err != nil {
		return 0, err
	}
	if !isFound {
		return 0, repo.ErrNotFound
	}
	return version, nil
}

// UpdateSysInfo 写入系统配置；用 UpdateColumns 不改 updated_at，它只记录用户的修改
func (r *repository) UpdateSysInfo(ctx context.Context, serverID uint64, fields SysInfoFields) error {
	result := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("id = ?", serverID).
		UpdateColumns(map[string]any{
			"cpu":     fields.CPU,
			"ram":     fields.RAM,
			"disk":    fields.Disk,
			"os_name": fields.OSName,
			"arch":    fields.Arch,
		})
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	// 部分数据库在值未变化时不计入影响行数，再查一次区分是否存在
	server, err := r.FindByID(ctx, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return repo.ErrNotFound
	}
	return nil
}

func (r *repository) getDB(ctx context.Context) *gorm.DB {
	return repo.GetDBFromCtx(ctx, r.db)
}
