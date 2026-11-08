package agentcommand

import (
	"context"
	"errors"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/protocol"
	taskrepo "game-panel/internal/repository/task"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeAgent struct {
	isOnline bool
	sendErr  error
	sent     []string
}

func (a *fakeAgent) IsOnline(uint64) bool { return a.isOnline }
func (a *fakeAgent) Send(_ uint64, commandID string, _ protocol.CmdPayload) error {
	a.sent = append(a.sent, commandID)
	return a.sendErr
}

type fakeNotifier struct{ events []string }

func (n *fakeNotifier) Publish(_ uint64, eventType string, _ any) {
	n.events = append(n.events, eventType)
}

func newTestService(t *testing.T) (*service, *fakeAgent, taskrepo.Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{TranslateError: true}) // 与 platform/database 一致
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Task{}, &entity.TaskLog{}); err != nil {
		t.Fatal(err)
	}
	repo := taskrepo.NewRepository(db)
	agent := &fakeAgent{isOnline: true}
	return NewService(repo, agent, &fakeNotifier{}).(*service), agent, repo
}

func deployCmd() Command {
	return Command{
		UserID: 1, ServerID: 7, TaskType: entity.TaskTypeGameDeploy, TaskName: "部署饥荒",
		ResourceType: entity.TaskResourceServer, ResourceID: 7,
		Params: map[string]string{"gameCode": "dst"}, Payload: protocol.CmdPayload{Action: protocol.ActionDeploy},
	}
}

func bizCode(err error) int {
	var be *apperr.BizError
	if errors.As(err, &be) {
		return be.Code
	}
	return 0
}

func TestDispatchSucceeded(t *testing.T) {
	s, _, repo := newTestService(t)
	ctx := context.Background()

	var handled *entity.Task
	s.OnSucceeded(entity.TaskTypeGameDeploy, func(_ context.Context, task *entity.Task) error {
		handled = task
		return nil
	})

	task, err := s.Dispatch(ctx, deployCmd())
	if err != nil {
		t.Fatal(err)
	}
	// 进行中时同一台服务器不能再下发
	if _, err := s.Dispatch(ctx, deployCmd()); bizCode(err) != 409 {
		t.Fatalf("重复下发应返回 409，got %v", err)
	}

	s.OnAck(7, task.CommandID, protocol.Ack{IsAccepted: true})
	s.OnAck(7, task.CommandID, protocol.Ack{IsAccepted: true}) // 重复确认不出错
	got, _ := repo.FindByID(ctx, task.ID)
	if got.Status != entity.TaskStatusRunning || got.Stage != entity.TaskStageExecuting {
		t.Fatalf("确认后 status=%s stage=%s", got.Status, got.Stage)
	}

	s.OnLog(7, task.CommandID, protocol.Log{Content: "[INFO] 开始部署"})
	logs, _ := repo.ListLogs(ctx, task.ID, 0, 10)
	if len(logs) != 1 || logs[0].Content != "[INFO] 开始部署" {
		t.Fatalf("logs = %+v", logs)
	}

	s.OnResult(7, task.CommandID, protocol.Result{Action: protocol.ActionDeploy, IsSuccess: true})
	got, _ = repo.FindByID(ctx, task.ID)
	if got.Status != entity.TaskStatusSucceeded || got.ActiveKey != nil {
		t.Fatalf("结束后 status=%s activeKey=%v", got.Status, got.ActiveKey)
	}
	if handled == nil || handled.Params["gameCode"] != "dst" {
		t.Fatalf("成功处理未执行或参数丢失：%+v", handled)
	}

	// 结束后释放了唯一键，可以再次下发
	if _, err := s.Dispatch(ctx, deployCmd()); err != nil {
		t.Fatalf("结束后应可再次下发：%v", err)
	}
}

func TestDispatchFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("Agent 离线", func(t *testing.T) {
		s, agent, _ := newTestService(t)
		agent.isOnline = false
		if _, err := s.Dispatch(ctx, deployCmd()); bizCode(err) != 409 {
			t.Fatalf("got %v", err)
		}
		if len(agent.sent) != 0 {
			t.Fatal("离线时不应下发")
		}
	})

	t.Run("下发失败", func(t *testing.T) {
		s, agent, repo := newTestService(t)
		agent.sendErr = errors.New("断开")
		if _, err := s.Dispatch(ctx, deployCmd()); bizCode(err) != 409 {
			t.Fatalf("got %v", err)
		}
		tasks, _ := repo.ListActive(ctx, 1)
		if len(tasks) != 0 {
			t.Fatal("下发失败的任务应已结束")
		}
	})

	t.Run("Agent 拒绝", func(t *testing.T) {
		s, _, repo := newTestService(t)
		task, _ := s.Dispatch(ctx, deployCmd())
		s.OnAck(7, task.CommandID, protocol.Ack{IsAccepted: false, Reason: "已有命令在执行"})
		got, _ := repo.FindByID(ctx, task.ID)
		if got.Status != entity.TaskStatusFailed || got.ErrorMessage != "Agent 拒绝执行：已有命令在执行" {
			t.Fatalf("status=%s msg=%s", got.Status, got.ErrorMessage)
		}
	})

	t.Run("连接中断后迟到的结果被忽略", func(t *testing.T) {
		s, _, repo := newTestService(t)
		task, _ := s.Dispatch(ctx, deployCmd())
		s.OnAck(7, task.CommandID, protocol.Ack{IsAccepted: true})
		s.OnInterrupted(7, task.CommandID, "断开")
		s.OnResult(7, task.CommandID, protocol.Result{IsSuccess: true})
		got, _ := repo.FindByID(ctx, task.ID)
		if got.Status != entity.TaskStatusFailed {
			t.Fatalf("status=%s", got.Status)
		}
	})

	t.Run("成功处理出错时任务失败", func(t *testing.T) {
		s, _, repo := newTestService(t)
		s.OnSucceeded(entity.TaskTypeGameDeploy, func(context.Context, *entity.Task) error { return errors.New("写库失败") })
		task, _ := s.Dispatch(ctx, deployCmd())
		s.OnResult(7, task.CommandID, protocol.Result{IsSuccess: true})
		got, _ := repo.FindByID(ctx, task.ID)
		if got.Status != entity.TaskStatusFailed {
			t.Fatalf("status=%s", got.Status)
		}
	})

	t.Run("平台重启后待执行的命令标记失败", func(t *testing.T) {
		s, _, repo := newTestService(t)
		task, _ := s.Dispatch(ctx, deployCmd())
		if err := s.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.FindByID(ctx, task.ID)
		if got.Status != entity.TaskStatusFailed || got.ActiveKey != nil {
			t.Fatalf("status=%s", got.Status)
		}
	})
}
