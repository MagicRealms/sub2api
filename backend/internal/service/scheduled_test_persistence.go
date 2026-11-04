package service

import (
	"context"
	"time"
)

const scheduledTestPersistenceTimeout = 15 * time.Second

func scheduledTestPersistenceContext(testCtx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(testCtx), scheduledTestPersistenceTimeout)
}
