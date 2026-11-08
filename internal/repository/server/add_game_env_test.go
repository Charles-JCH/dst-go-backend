package server

import (
	"context"
	"game-panel/internal/model/entity"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAddGameEnv(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{TranslateError: true}) // 与 platform/database 一致
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Server{}); err != nil {
		t.Fatal(err)
	}
	s := &entity.Server{Name: "a", IP: "1.1.1.1", Port: 22, Username: "root", Password: "x", GameEnv: []entity.GameEnvRef{}, UserID: 1}
	if err := db.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	ctx := context.Background()
	dst := entity.GameEnvRef{Code: "dst", Name: "饥荒联机版"}
	for range 2 { // 重复添加只保留一个
		if err := r.AddGameEnv(ctx, s.ID, dst); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := r.FindByID(ctx, s.ID)
	if len(got.GameEnv) != 1 || got.GameEnv[0] != dst {
		t.Fatalf("GameEnv = %+v", got.GameEnv)
	}
	if err := r.AddGameEnv(ctx, 999, dst); err == nil {
		t.Fatal("服务器不存在时应返回错误")
	}
}
