package runbook

import "testing"

func TestClassifyPage(t *testing.T) {
	tests := []struct {
		name     string
		analysis *SiteAnalysis
		want     string
	}{
		{
			name: "table when a container is tbody>tr",
			analysis: &SiteAnalysis{Containers: []ContainerCandidate{
				{Selector: ".item", Count: 4},
				{Selector: "tbody > tr", Count: 10},
			}},
			want: "table",
		},
		{
			name: "form with password field",
			analysis: &SiteAnalysis{Forms: []FormCandidate{
				{Selector: "form", Fields: []FormFieldCandidate{{Name: "p", Type: "password"}}},
			}},
			want: "form",
		},
		{
			name: "form with username field",
			analysis: &SiteAnalysis{Forms: []FormCandidate{
				{Selector: "form", Fields: []FormFieldCandidate{{Name: "username", Type: "text"}}},
			}},
			want: "form",
		},
		{
			name: "form with email field",
			analysis: &SiteAnalysis{Forms: []FormCandidate{
				{Selector: "form", Fields: []FormFieldCandidate{{Name: "email", Type: "text"}}},
			}},
			want: "form",
		},
		{
			name: "generic form (no login fields, no containers)",
			analysis: &SiteAnalysis{Forms: []FormCandidate{
				{Selector: "form", Fields: []FormFieldCandidate{{Name: "comment", Type: "text"}}},
			}},
			want: "form",
		},
		{
			name: "listing: containers with >=3 and no login form",
			analysis: &SiteAnalysis{Containers: []ContainerCandidate{
				{Selector: ".card", Count: 6},
			}},
			want: "listing",
		},
		{
			name: "form with non-login fields but containers present -> listing",
			analysis: &SiteAnalysis{
				Containers: []ContainerCandidate{{Selector: ".card", Count: 6}},
				Forms:      []FormCandidate{{Selector: "form", Fields: []FormFieldCandidate{{Name: "comment", Type: "text"}}}},
			},
			want: "listing",
		},
		{
			name: "article via og:type metadata",
			analysis: &SiteAnalysis{Metadata: map[string]string{"og:type": "article"}},
			want:     "article",
		},
		{
			name:     "unknown fallback",
			analysis: &SiteAnalysis{Metadata: map[string]string{}},
			want:     "unknown",
		},
		{
			name: "container under threshold and no form/meta -> unknown",
			analysis: &SiteAnalysis{
				Containers: []ContainerCandidate{{Selector: ".card", Count: 2}},
				Metadata:   map[string]string{},
			},
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.analysis.Metadata == nil {
				tt.analysis.Metadata = map[string]string{}
			}

			if got := classifyPage(tt.analysis); got != tt.want {
				t.Errorf("classifyPage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnalyzeDefaultsAndOption(t *testing.T) {
	o := analyzeDefaults()
	if o.maxContainers != 5 {
		t.Errorf("default maxContainers = %d, want 5", o.maxContainers)
	}

	WithMaxContainers(2)(o)
	if o.maxContainers != 2 {
		t.Errorf("after WithMaxContainers(2), maxContainers = %d, want 2", o.maxContainers)
	}
}
