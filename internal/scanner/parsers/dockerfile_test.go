package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerfileParser(t *testing.T) {
	parser := NewDockerfileParser()
	assert.NotNil(t, parser, "Should create a new DockerfileParser")
	assert.IsType(t, &DockerfileParser{}, parser, "Should return correct type")
}

func TestParseDockerfile(t *testing.T) {
	parser := NewDockerfileParser()

	tests := []struct {
		name         string
		content      string
		expectedInfo *DockerfileInfo
	}{
		{
			name: "basic Dockerfile",
			content: `FROM node:18-alpine
WORKDIR /app
COPY . .
RUN npm install
EXPOSE 3000
CMD ["npm", "start"]`,
			expectedInfo: &DockerfileInfo{
				BaseImages:   []string{"node:18-alpine"},
				ExposedPorts: []int{3000},
				MultiStage:   false,
				Stages:       []string{},
			},
		},
		{
			name: "multi-stage Dockerfile",
			content: `FROM node:18-alpine AS builder
WORKDIR /app
COPY . .
RUN npm install && npm run build

FROM nginx:alpine AS production
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80`,
			expectedInfo: &DockerfileInfo{
				BaseImages:   []string{"node:18-alpine", "nginx:alpine"},
				ExposedPorts: []int{80},
				MultiStage:   true,
				Stages:       []string{"builder", "production"},
			},
		},
		{
			name: "Dockerfile with multiple EXPOSE",
			content: `FROM python:3.9
WORKDIR /app
EXPOSE 8000
EXPOSE 8080 9000
CMD ["python", "app.py"]`,
			expectedInfo: &DockerfileInfo{
				BaseImages:   []string{"python:3.9"},
				ExposedPorts: []int{8000, 8080, 9000},
				MultiStage:   false,
				Stages:       []string{},
			},
		},
		{
			name: "Dockerfile with comments",
			content: `# Base image
FROM golang:1.21-alpine
# Set working directory
WORKDIR /app
# Expose port
EXPOSE 8080
CMD ["./app"]`,
			expectedInfo: &DockerfileInfo{
				BaseImages:   []string{"golang:1.21-alpine"},
				ExposedPorts: []int{8080},
				MultiStage:   false,
				Stages:       []string{},
			},
		},
		{
			name:         "empty Dockerfile",
			content:      "",
			expectedInfo: nil,
		},
		{
			name: "Dockerfile without FROM",
			content: `WORKDIR /app
COPY . .
RUN npm install`,
			expectedInfo: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseDockerfile(tt.content)

			if tt.expectedInfo == nil {
				assert.Nil(t, result, "Should return nil for invalid Dockerfile")
				return
			}

			require.NotNil(t, result, "Should return DockerfileInfo")
			assert.Equal(t, tt.expectedInfo.BaseImages, result.BaseImages, "Should have correct base images")
			assert.Equal(t, tt.expectedInfo.ExposedPorts, result.ExposedPorts, "Should have correct exposed ports")
			assert.Equal(t, tt.expectedInfo.MultiStage, result.MultiStage, "Should have correct multi-stage flag")
			assert.Equal(t, tt.expectedInfo.Stages, result.Stages, "Should have correct stages")
		})
	}
}

// TestParseDockerfile_VariableAndStageFrom verifies that a FROM referencing a
// build ARG is resolved from its default (or skipped when unresolvable) and
// that a FROM referencing a prior build stage is not treated as a registry
// image. Otherwise an unresolved "$VAR" or a stage name would leak into the
// SBOM as a bogus component.
func TestParseDockerfile_VariableAndStageFrom(t *testing.T) {
	parser := NewDockerfileParser()
	content := `ARG BUILD_IMAGE=node:18-alpine
FROM $BUILD_IMAGE AS builder
RUN npm ci
FROM builder AS test
FROM ${UNDEFINED_VAR}
FROM --platform=linux/amd64 nginx:1.25 AS run
`
	info := parser.ParseDockerfile(content)
	require.NotNil(t, info)

	// $BUILD_IMAGE -> node:18-alpine; "builder" (stage) and ${UNDEFINED_VAR}
	// excluded; nginx kept (platform flag stripped).
	assert.Equal(t, []string{"node:18-alpine", "nginx:1.25"}, info.BaseImages)
	assert.Equal(t, []string{"builder", "test", "run"}, info.Stages)
	assert.True(t, info.MultiStage)

	deps := parser.CreateDependencies(info)
	got := map[string]string{}
	for _, d := range deps {
		got[d.Name] = d.Version
	}
	assert.Equal(t, "18-alpine", got["node"])
	assert.Equal(t, "1.25", got["nginx"])
	if _, ok := got["$BUILD_IMAGE"]; ok {
		t.Error("unresolved $BUILD_IMAGE must not be emitted")
	}
	if _, ok := got["builder"]; ok {
		t.Error("build-stage reference must not be emitted as an image")
	}
}
