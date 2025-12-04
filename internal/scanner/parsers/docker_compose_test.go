package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerComposeParser(t *testing.T) {
	parser := NewDockerComposeParser()
	assert.NotNil(t, parser, "Should create a new DockerComposeParser")
	assert.IsType(t, &DockerComposeParser{}, parser, "Should return correct type")
}

func TestParseDockerCompose(t *testing.T) {
	parser := NewDockerComposeParser()

	tests := []struct {
		name             string
		content          string
		expectedServices []DockerService
	}{
		{
			name: "basic docker-compose.yml",
			content: `version: '3.8'
services:
  web:
    image: nginx:latest
    container_name: web-server
  db:
    image: postgres:13
    container_name: postgres-db
`,
			expectedServices: []DockerService{
				{Name: "web", Image: "nginx:latest", ContainerName: "web-server"},
				{Name: "db", Image: "postgres:13", ContainerName: "postgres-db"},
			},
		},
		{
			name: "docker-compose with only images",
			content: `version: '3.8'
services:
  app:
    image: myapp:1.0.0
  cache:
    image: redis:alpine
  db:
    image: mysql:8.0
`,
			expectedServices: []DockerService{
				{Name: "app", Image: "myapp:1.0.0", ContainerName: ""},
				{Name: "cache", Image: "redis:alpine", ContainerName: ""},
				{Name: "db", Image: "mysql:8.0", ContainerName: ""},
			},
		},
		{
			name: "docker-compose with no services",
			content: `version: '3.8'
volumes:
  data:
    driver: local
`,
			expectedServices: []DockerService{},
		},
		{
			name:             "empty docker-compose",
			content:          "",
			expectedServices: []DockerService{},
		},
		{
			name: "docker-compose with comments and empty lines",
			content: `# Docker Compose file
version: '3.8'

services:
  # Web service
  web:
    image: nginx:latest
    
  # Database service
  db:
    image: postgres:13
    # Container name
    container_name: postgres-db

# End of file
`,
			expectedServices: []DockerService{
				{Name: "web", Image: "nginx:latest", ContainerName: ""},
				{Name: "db", Image: "postgres:13", ContainerName: "postgres-db"},
			},
		},
		{
			name: "docker-compose with quoted images",
			content: `services:
  app:
    image: "myapp:1.0.0"
    container_name: "my-app-container"
  api:
    image: 'myapi:2.0.0'
`,
			expectedServices: []DockerService{
				{Name: "app", Image: "myapp:1.0.0", ContainerName: "my-app-container"},
				{Name: "api", Image: "myapi:2.0.0", ContainerName: ""},
			},
		},
		{
			name: "docker-compose with complex structure",
			content: `version: '3.8'
services:
  web:
    image: nginx:latest
    container_name: web-server
    ports:
      - "80:80"
    environment:
      - NODE_ENV=production
  db:
    image: postgres:13
    container_name: postgres-db
    environment:
      - POSTGRES_DB=myapp
      - POSTGRES_USER=admin
    volumes:
      - db_data:/var/lib/postgresql/data
  redis:
    image: redis:alpine
    command: redis-server --appendonly yes
volumes:
  db_data:
`,
			expectedServices: []DockerService{
				{Name: "web", Image: "nginx:latest", ContainerName: "web-server"},
				{Name: "db", Image: "postgres:13", ContainerName: "postgres-db"},
				{Name: "redis", Image: "redis:alpine", ContainerName: ""},
			},
		},
		{
			name: "docker-compose with nested services",
			content: `services:
  frontend:
    image: nginx:latest
    container_name: frontend-server
  backend:
    image: node:16
    container_name: backend-server
    environment:
      - NODE_ENV=production
    depends_on:
      - db
      - redis
  db:
    image: postgres:13
    container_name: postgres-db
  redis:
    image: redis:alpine
    container_name: redis-cache
`,
			expectedServices: []DockerService{
				{Name: "frontend", Image: "nginx:latest", ContainerName: "frontend-server"},
				{Name: "backend", Image: "node:16", ContainerName: "backend-server"},
				{Name: "db", Image: "postgres:13", ContainerName: "postgres-db"},
				{Name: "redis", Image: "redis:alpine", ContainerName: "redis-cache"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseDockerCompose(tt.content)

			require.Len(t, result, len(tt.expectedServices), "Should return correct number of services")

			for i, expectedService := range tt.expectedServices {
				assert.Equal(t, expectedService.Name, result[i].Name, "Should have correct service name")
				assert.Equal(t, expectedService.Image, result[i].Image, "Should have correct image")
				assert.Equal(t, expectedService.ContainerName, result[i].ContainerName, "Should have correct container name")
			}
		})
	}
}

func TestParseDockerCompose_EdgeCases(t *testing.T) {
	parser := NewDockerComposeParser()

	tests := []struct {
		name             string
		content          string
		expectedServices []DockerService
	}{
		{
			name: "services with no images",
			content: `services:
  web:
    container_name: web-server
    ports:
      - "80:80"
  db:
    container_name: postgres-db
`,
			expectedServices: []DockerService{
				{Name: "web", Image: "", ContainerName: "web-server"},
				{Name: "db", Image: "", ContainerName: "postgres-db"},
			},
		},
		{
			name: "services without proper indentation",
			content: `services:
web:
  image: nginx:latest
db:
  image: postgres:13
`,
			expectedServices: []DockerService{}, // Should not parse incorrectly indented services
		},
		{
			name: "services section but no services defined",
			content: `version: '3.8'
services:
# No services defined here
`,
			expectedServices: []DockerService{},
		},
		{
			name: "docker-compose with multiple top-level sections",
			content: `version: '3.8'
networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge

services:
  web:
    image: nginx:latest
  db:
    image: postgres:13

volumes:
  data:
    driver: local
`,
			expectedServices: []DockerService{
				{Name: "web", Image: "nginx:latest", ContainerName: ""},
				{Name: "db", Image: "postgres:13", ContainerName: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseDockerCompose(tt.content)

			require.Len(t, result, len(tt.expectedServices), "Should return correct number of services")

			for i, expectedService := range tt.expectedServices {
				assert.Equal(t, expectedService.Name, result[i].Name, "Should have correct service name")
				assert.Equal(t, expectedService.Image, result[i].Image, "Should have correct image")
				assert.Equal(t, expectedService.ContainerName, result[i].ContainerName, "Should have correct container name")
			}
		})
	}
}

func TestParseDockerCompose_ImageFormats(t *testing.T) {
	parser := NewDockerComposeParser()

	tests := []struct {
		name             string
		content          string
		expectedServices []DockerService
	}{
		{
			name: "images with tags and digests",
			content: `services:
  app1:
    image: nginx:latest
  app2:
    image: postgres:13-alpine
  app3:
    image: redis@sha256:abc123
  app4:
    image: myregistry.com/myapp:1.0.0
`,
			expectedServices: []DockerService{
				{Name: "app1", Image: "nginx:latest", ContainerName: ""},
				{Name: "app2", Image: "postgres:13-alpine", ContainerName: ""},
				{Name: "app3", Image: "redis@sha256:abc123", ContainerName: ""},
				{Name: "app4", Image: "myregistry.com/myapp:1.0.0", ContainerName: ""},
			},
		},
		{
			name: "images with whitespace and quotes",
			content: `services:
  app:
    image:    nginx:latest    
    container_name:   "web-server"   
  db:
    image:   "postgres:13"   
    container_name:  'postgres-db' 
`,
			expectedServices: []DockerService{
				{Name: "app", Image: "nginx:latest", ContainerName: "web-server"},
				{Name: "db", Image: "postgres:13", ContainerName: "postgres-db"},
			},
		},
		{
			name: "comprehensive quote handling test",
			content: `services:
  double-quoted-image:
    image: "nginx:latest"
    container_name: "nginx-server"
  single-quoted-image:
    image: 'postgres:13'
    container_name: 'postgres-db'
  mixed-quotes:
    image: "redis:alpine"
    container_name: 'redis-cache'
  no-quotes:
    image: mysql:8.0
    container_name: mysql-db
`,
			expectedServices: []DockerService{
				{Name: "double-quoted-image", Image: "nginx:latest", ContainerName: "nginx-server"},
				{Name: "single-quoted-image", Image: "postgres:13", ContainerName: "postgres-db"},
				{Name: "mixed-quotes", Image: "redis:alpine", ContainerName: "redis-cache"},
				{Name: "no-quotes", Image: "mysql:8.0", ContainerName: "mysql-db"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseDockerCompose(tt.content)

			require.Len(t, result, len(tt.expectedServices), "Should return correct number of services")

			for i, expectedService := range tt.expectedServices {
				assert.Equal(t, expectedService.Name, result[i].Name, "Should have correct service name")
				assert.Equal(t, expectedService.Image, result[i].Image, "Should have correct image")
				assert.Equal(t, expectedService.ContainerName, result[i].ContainerName, "Should have correct container name")
			}
		})
	}
}

func TestDockerComposeParser_Integration(t *testing.T) {
	parser := NewDockerComposeParser()

	// Test realistic docker-compose.yml
	realisticCompose := `version: '3.8'

# Docker Compose for a full-stack application
services:
  # Frontend service
  frontend:
    image: nginx:alpine
    container_name: frontend-server
    ports:
      - "80:80"
    depends_on:
      - backend

  # Backend service
  backend:
    image: myapp/backend:2.1.0
    container_name: backend-server
    environment:
      - NODE_ENV=production
      - DATABASE_URL=postgresql://user:pass@db:5432/myapp
    depends_on:
      - db
      - redis

  # Database service
  db:
    image: postgres:13-alpine
    container_name: postgres-db
    environment:
      - POSTGRES_DB=myapp
      - POSTGRES_USER=admin
      - POSTGRES_PASSWORD=secret
    volumes:
      - postgres_data:/var/lib/postgresql/data

  # Redis cache
  redis:
    image: redis:alpine
    container_name: redis-cache
    command: redis-server --appendonly yes

  # Database admin tool
  adminer:
    image: adminer:latest
    container_name: db-admin
    ports:
      - "8080:8080"
    depends_on:
      - db

# Networks
networks:
  default:
    driver: bridge

# Volumes
volumes:
  postgres_data:
    driver: local
`

	services := parser.ParseDockerCompose(realisticCompose)
	assert.Len(t, services, 5, "Should parse 5 services")

	// Create service map for verification
	serviceMap := make(map[string]DockerService)
	for _, service := range services {
		serviceMap[service.Name] = service
	}

	// Verify specific services
	assert.Equal(t, "nginx:alpine", serviceMap["frontend"].Image)
	assert.Equal(t, "frontend-server", serviceMap["frontend"].ContainerName)

	assert.Equal(t, "myapp/backend:2.1.0", serviceMap["backend"].Image)
	assert.Equal(t, "backend-server", serviceMap["backend"].ContainerName)

	assert.Equal(t, "postgres:13-alpine", serviceMap["db"].Image)
	assert.Equal(t, "postgres-db", serviceMap["db"].ContainerName)

	assert.Equal(t, "redis:alpine", serviceMap["redis"].Image)
	assert.Equal(t, "redis-cache", serviceMap["redis"].ContainerName)

	assert.Equal(t, "adminer:latest", serviceMap["adminer"].Image)
	assert.Equal(t, "db-admin", serviceMap["adminer"].ContainerName)
}

func TestDockerComposeParser_ErrorHandling(t *testing.T) {
	parser := NewDockerComposeParser()

	// Test malformed YAML that still parses
	malformedContent := `services:
  web:
    image: nginx:latest
    container_name: web-server
  db:
    image: postgres:13
    container_name: postgres-db
# Missing closing sections should still parse services`

	services := parser.ParseDockerCompose(malformedContent)
	assert.Len(t, services, 2, "Should still parse services despite malformed structure")
	assert.Equal(t, "web", services[0].Name)
	assert.Equal(t, "db", services[1].Name)
}
