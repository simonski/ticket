package store

import "testing"

func TestGuidanceMapResolveUsesStageSpecificValue(t *testing.T) {
	t.Parallel()

	dorMap := GuidanceMap{
		"default": "work is described clearly enough to begin",
		"develop": "design is approved and dependencies are understood",
	}

	got, ok := dorMap.Resolve("develop")
	if !ok {
		t.Fatal("Resolve(develop) found = false, want true")
	}
	if got != "design is approved and dependencies are understood" {
		t.Fatalf("Resolve(develop) = %q", got)
	}
}

func TestGuidanceMapResolveFallsBackToDefault(t *testing.T) {
	t.Parallel()

	dorMap := GuidanceMap{
		"default": "work is described clearly enough to begin",
	}

	got, ok := dorMap.Resolve("test")
	if !ok {
		t.Fatal("Resolve(test) found = false, want true")
	}
	if got != "work is described clearly enough to begin" {
		t.Fatalf("Resolve(test) = %q", got)
	}
}

func TestGuidanceMapResolveReturnsNoValueWhenStageAndDefaultMissing(t *testing.T) {
	t.Parallel()

	dorMap := GuidanceMap{
		"design": "problem statement is understood",
	}

	got, ok := dorMap.Resolve("develop")
	if ok {
		t.Fatalf("Resolve(develop) found = true, want false with value %q", got)
	}
	if got != "" {
		t.Fatalf("Resolve(develop) = %q, want empty string", got)
	}
}

func TestResolveGuidancePreservesLayerIndependence(t *testing.T) {
	t.Parallel()

	project := Project{
		DORMap: GuidanceMap{
			"default": "work must map to a valid project objective",
		},
	}
	role := Role{
		DORMap: GuidanceMap{
			"develop": "requirements and boundaries are clear enough to implement",
		},
	}

	projectResolved := project.ResolveGuidance("develop")
	roleResolved := role.ResolveGuidance("develop")

	if !projectResolved.HasDOR || projectResolved.DOR != "work must map to a valid project objective" {
		t.Fatalf("project ResolveGuidance(develop) = %#v", projectResolved)
	}
	if !roleResolved.HasDOR || roleResolved.DOR != "requirements and boundaries are clear enough to implement" {
		t.Fatalf("role ResolveGuidance(develop) = %#v", roleResolved)
	}
}

func TestGuidanceMapResolveMatchesStageCaseInsensitively(t *testing.T) {
	t.Parallel()

	dorMap := GuidanceMap{
		"Develop": "design is approved and dependencies are understood",
	}

	got, ok := dorMap.Resolve(" develop ")
	if !ok {
		t.Fatal("Resolve( develop ) found = false, want true")
	}
	if got != "design is approved and dependencies are understood" {
		t.Fatalf("Resolve( develop ) = %q", got)
	}
}
