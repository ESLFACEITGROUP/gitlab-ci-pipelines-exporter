package controller

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/xanzy/go-gitlab"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mvisonneau/gitlab-ci-pipelines-exporter/pkg/config"
	"github.com/mvisonneau/gitlab-ci-pipelines-exporter/pkg/schemas"
)

func TestTriggerRefMetricsPull(t *testing.T) {
	ctx, c, _, srv := newTestController(config.Config{})
	srv.Close()

	ref1 := schemas.Ref{
		Project: schemas.NewProject("group/foo"),
		Name:    "main",
	}

	p2 := schemas.NewProject("group/bar")
	ref2 := schemas.Ref{
		Project: p2,
		Name:    "main",
	}

	assert.NoError(t, c.Store.SetRef(ctx, ref1))
	assert.NoError(t, c.Store.SetProject(ctx, p2))

	// TODO: Assert results somehow
	c.triggerRefMetricsPull(ctx, ref1)
	c.triggerRefMetricsPull(ctx, ref2)
}

func TestTriggerEnvironmentMetricsPull(t *testing.T) {
	ctx, c, _, srv := newTestController(config.Config{})
	srv.Close()

	p1 := schemas.NewProject("foo/bar")
	env1 := schemas.Environment{
		ProjectName: p1.Name,
		Name:        "dev",
	}

	env2 := schemas.Environment{
		ProjectName: "foo/baz",
		Name:        "prod",
	}

	assert.NoError(t, c.Store.SetProject(ctx, p1))
	assert.NoError(t, c.Store.SetEnvironment(ctx, env1))
	assert.NoError(t, c.Store.SetEnvironment(ctx, env2))

	// TODO: Assert results somehow
	c.triggerEnvironmentMetricsPull(ctx, env1)
	c.triggerEnvironmentMetricsPull(ctx, env2)
}

func TestController_processJobEvent(t *testing.T) {
	ctx, c, mux, srv := newTestController(config.Config{
		Wildcards: config.Wildcards{
			{
				Search: "*",
				Owner: config.WildcardOwner{
					Name:             "foo",
					Kind:             "group",
					IncludeSubgroups: false,
				},
				Archived: false,
				ProjectParameters: config.ProjectParameters{
					Pull: config.ProjectPull{
						Refs: config.ProjectPullRefs{
							Branches: config.ProjectPullRefsBranches{
								Enabled:        true,
								Regexp:         "master",
								MostRecent:     1,
								MaxAgeSeconds:  0,
								ExcludeDeleted: true,
							},
							Tags:          config.ProjectPullRefsTags{},
							MergeRequests: config.ProjectPullRefsMergeRequests{},
						},
					},
					OutputSparseStatusMetrics: false,
				},
			},
		},
	})
	defer srv.Close()

	mux.HandleFunc("/api/v4/projects/380",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id":380,"path_with_namespace":"foo/bar","jobs_enabled":true}`)
		})

	val, err := readJobEvent(t, "../../testdata/webhook/job_events.json")
	require.NoError(t, err)

	err = c.processJobEvent(ctx, *val)
	assert.NoError(t, err)

	// Validate that the pull project task was queued
	n, err := c.Store.CurrentlyQueuedTasksCount(ctx)
	assert.Equal(t, uint64(1), n)
}

func readJobEvent(t *testing.T, filePath string) (*gitlab.JobEvent, error) {
	t.Helper()
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

	var jobEnvent gitlab.JobEvent
	err = json.Unmarshal(byteValue, &jobEnvent)
	if err != nil {
		return nil, err
	}

	return &jobEnvent, nil
}
