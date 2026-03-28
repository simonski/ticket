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
	for _, state := range []string{StateIdle, StateActive, StateSuccess, StateFail, StateComplete} {
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
		{StageTest, StateSuccess, true},
		{StageDone, StateSuccess, true},
		{StageDone, StateFail, true},
		{StageDone, StateComplete, true},
		{StageDone, StateIdle, false},
		{StageDone, StateActive, false},
		{StageDesign, StateSuccess, true},
		{StageDesign, StateFail, true},
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

func TestParseLifecycleStatus(t *testing.T) {
	cases := []struct {
		input     string
		wantStage string
		wantState string
		wantErr   bool
	}{
		{"design/idle", StageDesign, StateIdle, false},
		{"develop/active", StageDevelop, StateActive, false},
		{"test/success", StageTest, StateSuccess, false},
		{"done/complete", StageDone, StateSuccess, false},
		{"DESIGN/IDLE", StageDesign, StateIdle, false},
		{"", "", "", true},
		{"noslash", "", "", true},
		{"bad/open", "", "", true},
		{"open/idle", "", "", true},
	}
	for _, tc := range cases {
		stage, state, err := ParseLifecycleStatus(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ParseLifecycleStatus(%q) error = nil, want error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLifecycleStatus(%q) error = %v", tc.input, err)
		}
		if stage != tc.wantStage || state != tc.wantState {
			t.Fatalf("ParseLifecycleStatus(%q) = (%q, %q), want (%q, %q)", tc.input, stage, state, tc.wantStage, tc.wantState)
		}
	}
}
