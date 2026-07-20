package api

import (
	"context"
	"log"
	"time"
)

type expiredInstance struct {
	ID         int64
	Backend    string
	TargetID   string
	InstanceID string
}

func (s *Server) StartInstanceCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := s.CleanupExpiredInstances(ctx); err != nil {
					log.Printf("warning: expired instance cleanup failed: %v", err)
				} else if n > 0 {
					log.Printf("expired instance cleanup stopped %d instance(s)", n)
				}
			}
		}
	}()
}

func (s *Server) CleanupExpiredInstances(ctx context.Context) (int, error) {
	rows, err := s.db.Query(`
		SELECT id, backend, target_id, instance_id
		FROM instances
		WHERE status='running' AND expires_at IS NOT NULL AND expires_at <= ?
		ORDER BY expires_at ASC
	`, time.Now())
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	expired := []expiredInstance{}
	for rows.Next() {
		var inst expiredInstance
		if err := rows.Scan(&inst.ID, &inst.Backend, &inst.TargetID, &inst.InstanceID); err != nil {
			return 0, err
		}
		expired = append(expired, inst)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	stopped := 0
	for _, inst := range expired {
		targetName := inst.TargetID
		if targetName == "" {
			targetName = inst.Backend
		}
		if back, err := s.deployer.Get(targetName); err == nil {
			if err := back.Destroy(ctx, inst.InstanceID); err != nil {
				_, _ = s.db.Exec(`UPDATE instances SET status='error' WHERE id=?`, inst.ID)
				continue
			}
		}
		if _, err := s.db.Exec(`UPDATE instances SET status='stopped' WHERE id=?`, inst.ID); err != nil {
			return stopped, err
		}
		stopped++
	}
	return stopped, nil
}
