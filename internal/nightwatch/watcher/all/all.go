package all

//nolint: golint
import (
	_ "github.com/ashwinyue/dcp/internal/nightwatch/watcher/cronjob/cronjob"
	_ "github.com/ashwinyue/dcp/internal/nightwatch/watcher/cronjob/statesync"
	_ "github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/llmtrain"
	_ "github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch"
)
