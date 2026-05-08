package checker

import (
	"testing"

	"weops-inspect/model"
)

func TestCheckBKDeps_OK(t *testing.T) {
	s := &model.BKMonitorV3Section{Dependencies: []model.DependencyResult{{Item: "redis", Status: "ok"}}}
	got := CheckBKDeps(s)
	if len(got) != 1 || got[0].Status != model.StatusOK {
		t.Fatalf("want ok, got %v", got)
	}
	if s.Dependencies[0].RenderStatus != model.StatusOK {
		t.Errorf("RenderStatus = %v", s.Dependencies[0].RenderStatus)
	}
}

func TestCheckBKDeps_FailNotice(t *testing.T) {
	s := &model.BKMonitorV3Section{Dependencies: []model.DependencyResult{{Item: "mysql", Status: "fail"}}}
	got := CheckBKDeps(s)
	if len(got) != 1 || got[0].Status != model.StatusNotice {
		t.Fatalf("want notice, got %v", got)
	}
}

func TestCheckBKDeps_SkipNoResult(t *testing.T) {
	s := &model.BKMonitorV3Section{Dependencies: []model.DependencyResult{{Item: "x", Status: "skip"}}}
	got := CheckBKDeps(s)
	if len(got) != 0 {
		t.Fatalf("skip should produce no result, got %v", got)
	}
}

func TestCheckBKDeps_Nil(t *testing.T) {
	if got := CheckBKDeps(nil); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}
