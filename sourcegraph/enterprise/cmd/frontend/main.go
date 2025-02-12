// Command frontend contains the enterprise frontend implementation.
//
// It wraps the open source frontend command and merely injects a few
// proprietary things into it via e.g. blank/underscore imports in this file
// which register side effects with the frontend package.
package main

import (
	"context"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/shared"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/app"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/authz"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/batches"
	codeintelinit "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codemonitors"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/compute"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/dotcom"
	executor "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/executorqueue"
	licensing "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/licensing/init"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/notebooks"
	_ "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/registry"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/searchcontexts"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/trace"
)

func main() {
	shared.Main(enterpriseSetupHook)
}

func init() {
	oobmigration.ReturnEnterpriseMigrations = true
}

type EnterpriseInitializer = func(context.Context, database.DB, codeintel.Services, conftypes.UnifiedWatchable, *enterprise.Services, *observation.Context) error

var initFunctions = map[string]EnterpriseInitializer{
	"app":            app.Init,
	"authz":          authz.Init,
	"batches":        batches.Init,
	"codeintel":      codeintelinit.Init,
	"codemonitors":   codemonitors.Init,
	"compute":        compute.Init,
	"dotcom":         dotcom.Init,
	"insights":       insights.Init,
	"licensing":      licensing.Init,
	"notebooks":      notebooks.Init,
	"searchcontexts": searchcontexts.Init,
}

func enterpriseSetupHook(db database.DB, codeIntelServices codeintel.Services, conf conftypes.UnifiedWatchable) enterprise.Services {
	logger := log.Scoped("enterprise", "frontend enterprise edition")
	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))
	if debug {
		logger.Debug("enterprise edition")
	}

	auth.Init(logger, db)

	ctx := context.Background()
	enterpriseServices := enterprise.DefaultServices()

	observationContext := &observation.Context{
		Logger:     logger,
		Tracer:     &trace.Tracer{TracerProvider: otel.GetTracerProvider()},
		Registerer: prometheus.DefaultRegisterer,
	}

	for name, fn := range initFunctions {
		if err := fn(ctx, db, codeIntelServices, conf, &enterpriseServices, observationContext); err != nil {
			logger.Fatal("failed to initialize", log.String("name", name), log.Error(err))
		}
	}

	// Inititalize executor last, as we require code intel and batch changes services to be
	// already populated on the enterpriseServices object.
	if err := executor.Init(ctx, db, conf, &enterpriseServices, observationContext); err != nil {
		logger.Fatal("failed to initialize executor", log.Error(err))
	}

	return enterpriseServices
}
