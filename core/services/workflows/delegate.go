package workflows

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pelletier/go-toml"

	"github.com/smartcontractkit/chainlink-common/pkg/custmsg"

	"github.com/smartcontractkit/chainlink-common/pkg/types/core"

	"github.com/smartcontractkit/chainlink/v2/core/logger"
	"github.com/smartcontractkit/chainlink/v2/core/services/job"
	"github.com/smartcontractkit/chainlink/v2/core/services/workflows/store"
)

type Delegate struct {
	registry       core.CapabilitiesRegistry
	secretsFetcher secretsFetcher
	logger         logger.Logger
	store          store.Store
}

var _ job.Delegate = (*Delegate)(nil)

func (d *Delegate) JobType() job.Type {
	return job.Workflow
}

func (d *Delegate) BeforeJobCreated(spec job.Job) {}

func (d *Delegate) AfterJobCreated(jb job.Job) {}

func (d *Delegate) BeforeJobDeleted(spec job.Job) {}

func (d *Delegate) OnDeleteJob(context.Context, job.Job) error { return nil }

// ServicesForSpec satisfies the job.Delegate interface.
func (d *Delegate) ServicesForSpec(ctx context.Context, spec job.Job) ([]job.ServiceCtx, error) {
	cma := custmsg.NewLabeler().With(wIDKey, spec.WorkflowSpec.WorkflowID, woIDKey, spec.WorkflowSpec.WorkflowOwner, wnKey, spec.WorkflowSpec.WorkflowName)
	sdkSpec, err := spec.WorkflowSpec.SDKSpec(ctx)
	if err != nil {
		logCustMsg(ctx, cma, fmt.Sprintf("failed to start workflow engine: failed to get workflow sdk spec: %v", err), d.logger)
		return nil, err
	}

	binary, err := spec.WorkflowSpec.RawSpec(ctx)
	if err != nil {
		logCustMsg(ctx, cma, fmt.Sprintf("failed to start workflow engine: failed to fetch workflow spec binary: %v", err), d.logger)
		return nil, err
	}

	config, err := spec.WorkflowSpec.GetConfig(ctx)
	if err != nil {
		logCustMsg(ctx, cma, fmt.Sprintf("failed to start workflow engine: failed to get workflow spec config: %v", err), d.logger)
		return nil, err
	}

	cfg := Config{
		Lggr:           d.logger,
		Workflow:       sdkSpec,
		WorkflowID:     spec.WorkflowSpec.WorkflowID,
		WorkflowOwner:  spec.WorkflowSpec.WorkflowOwner,
		WorkflowName:   spec.WorkflowSpec.WorkflowName,
		Registry:       d.registry,
		Store:          d.store,
		Config:         config,
		Binary:         binary,
		SecretsFetcher: d.secretsFetcher,
	}
	engine, err := NewEngine(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return []job.ServiceCtx{engine}, nil
}

func NewDelegate(
	logger logger.Logger,
	registry core.CapabilitiesRegistry,
	secretsFetcher secretsFetcher,
	store store.Store,
) *Delegate {
	return &Delegate{logger: logger, registry: registry, secretsFetcher: secretsFetcher, store: store}
}

func ValidatedWorkflowJobSpec(ctx context.Context, tomlString string) (job.Job, error) {
	var jb = job.Job{ExternalJobID: uuid.New()}

	tree, err := toml.Load(tomlString)
	if err != nil {
		return jb, fmt.Errorf("toml error on load: %w", err)
	}

	err = tree.Unmarshal(&jb)
	if err != nil {
		return jb, fmt.Errorf("toml unmarshal error on spec: %w", err)
	}
	if jb.Type != job.Workflow {
		return jb, fmt.Errorf("unsupported type %s, expected %s", jb.Type, job.Workflow)
	}

	var spec job.WorkflowSpec
	err = tree.Unmarshal(&spec)
	if err != nil {
		return jb, fmt.Errorf("toml unmarshal error on workflow spec: %w", err)
	}

	sdkSpec, err := spec.SDKSpec(ctx)
	if err != nil {
		return jb, fmt.Errorf("failed to convert to sdk workflow spec: %w", err)
	}

	// ensure the embedded workflow graph is valid
	if _, err = Parse(sdkSpec); err != nil {
		return jb, fmt.Errorf("failed to parse workflow graph: %w", err)
	}

	err = spec.Validate(ctx)
	if err != nil {
		return jb, fmt.Errorf("invalid WorkflowSpec: %w", err)
	}

	jb.WorkflowSpec = &spec
	jb.WorkflowSpecID = &spec.ID

	return jb, nil
}
