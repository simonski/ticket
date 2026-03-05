package store

import "testing"

func TestValidStage(t *testing.T) {
	for _, stage := range []string{StageDesign, StageDevelop, StageTest, StageDone} {
		if !ValidStage(stage) {
			t.Fatalf("ValidStage(%q) = false, want true", stage)
		}
	}
	if ValidStage("open") {
		t.Fatal(`ValidStage("open") = true, want false`)
	}
}

func TestValidState(t *testing.T) {
	for _, state := range []string{StateIdle, StateActive, StateComplete} {
		if !ValidState(state) {
			t.Fatalf("ValidState(%q) = false, want true", state)
		}
	}
	if ValidState("inprogress") {
		t.Fatal(`ValidState("inprogress") = true, want false`)
	}
}

func TestValidLifecycle(t *testing.T) {
	cases := []struct {
		stage string
		state string
		want  bool
	}{
		{StageDesign, StateIdle, true},
		{StageDevelop, StateActive, true},
		{StageTest, StateComplete, true},
		{StageDone, StateComplete, true},
		{StageDone, StateIdle, false},
		{StageDone, StateActive, false},
		{"open", StateIdle, false},
		{StageDesign, "open", false},
	}
	for _, tc := range cases {
		if got := ValidLifecycle(tc.stage, tc.state); got != tc.want {
			t.Fatalf("ValidLifecycle(%q, %q) = %t, want %t", tc.stage, tc.state, got, tc.want)
		}
	}
}

func TestRenderLifecycleStatus(t *testing.T) {
	if got := RenderLifecycleStatus(StageDesign, StateIdle); got != "design/idle" {
		t.Fatalf("RenderLifecycleStatus() = %q", got)
	}
}

func TestCompareStageOrder(t *testing.T) {
	if got := CompareStageOrder(StageDesign, StageDevelop); got >= 0 {
		t.Fatalf("CompareStageOrder(design, develop) = %d, want < 0", got)
	}
	if got := CompareStageOrder(StageDone, StageTest); got <= 0 {
		t.Fatalf("CompareStageOrder(done, test) = %d, want > 0", got)
	}
	if got := CompareStageOrder(StageTest, StageTest); got != 0 {
		t.Fatalf("CompareStageOrder(test, test) = %d, want 0", got)
	}
}
