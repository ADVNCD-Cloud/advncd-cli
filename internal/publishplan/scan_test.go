package publishplan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan_StackDetectionMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "nextjs",
			files: map[string]string{
				"package.json": `{"dependencies":{"next":"14.0.0"}}`,
			},
			want: StackNextJS,
		},
		{
			name: "nuxt",
			files: map[string]string{
				"package.json": `{"dependencies":{"nuxt":"3.0.0"}}`,
			},
			want: StackNuxt,
		},
		{
			name: "express",
			files: map[string]string{
				"package.json": `{"dependencies":{"express":"5.0.0"}}`,
			},
			want: StackExpress,
		},
		{
			name: "fastapi",
			files: map[string]string{
				"requirements.txt": "fastapi==0.116.0\nuvicorn==0.31.0\n",
			},
			want: StackFastAPI,
		},
		{
			name: "django",
			files: map[string]string{
				"manage.py": "#!/usr/bin/env python\n",
			},
			want: StackDjango,
		},
		{
			name: "spring-boot",
			files: map[string]string{
				"pom.xml": `<project><artifactId>demo</artifactId><dependency>spring-boot-starter-web</dependency></project>`,
			},
			want: StackSpringBoot,
		},
		{
			name: "dotnet",
			files: map[string]string{
				"api.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web"></Project>`,
			},
			want: StackDotNet,
		},
		{
			name: "symfony",
			files: map[string]string{
				"composer.json": `{"require":{"symfony/framework-bundle":"^7.0"}}`,
			},
			want: StackSymfony,
		},
		{
			name: "gin",
			files: map[string]string{
				"go.mod": "module x\n\nrequire github.com/gin-gonic/gin v1.10.0\n",
			},
			want: StackGin,
		},
		{
			name: "rails",
			files: map[string]string{
				"Gemfile": `gem "rails", "~> 7.1"`,
			},
			want: StackRails,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for rel, body := range tc.files {
				path := filepath.Join(dir, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatalf("write %s: %v", rel, err)
				}
			}

			p, err := Scan(dir)
			if err != nil {
				t.Fatalf("Scan error: %v", err)
			}
			if p.Stack != tc.want {
				t.Fatalf("stack mismatch: got %q want %q", p.Stack, tc.want)
			}
		})
	}
}

func TestScan_DefaultEnvForNodeLikeStack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	p, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if p.Env["NODE_ENV"] != "production" {
		t.Fatalf("missing NODE_ENV default")
	}
}
