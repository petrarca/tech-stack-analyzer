# LLM Enhancement Opportunities for Tech Stack Analyzer

## Overview

This document outlines potential use cases where Large Language Models (LLMs) could enhance the Tech Stack Analyzer's detection capabilities while maintaining the core principles of zero-dependency deployment, speed, and deterministic rule-based detection.

## Core Principle

**LLM enhancement is OPTIONAL and SUPPLEMENTARY to rule-based detection**, not a replacement. The existing 800+ rule-based detection system remains the primary, fast, and deterministic foundation.

## Architecture Philosophy

```
┌─────────────────────────────────────────┐
│   Rule-Based Detection (Primary)        │
│   - Fast: O(1) lookups                  │
│   - Deterministic: Same input = output  │
│   - Handles 95%+ cases                  │
└─────────────┬───────────────────────────┘
              │
              ▼
    ┌─────────────────────┐
    │  Confidence Check   │
    │  Ambiguity Detected?│
    └─────────┬───────────┘
              │
    ┌─────────▼──────────┐
    │   LLM Enhancement   │◄─── Optional, opt-in
    │   (Contextual)      │     Separate binary
    └─────────┬───────────┘     or plugin
              │
              ▼
    ┌──────────────────────┐
    │  Enhanced Results +  │
    │  Confidence Scores + │
    │  Recommendations     │
    └──────────────────────┘
```

---

## Post-Processing Architecture

### Why Post-Processing?

After evaluating integrated vs. post-processing approaches for LLM enhancement, **post-processing is the recommended architecture**. Here's why:

| Criterion | Post-Processing | Integrated |
|-----------|-----------------|------------|
| **Performance** | ✅ No impact on scan speed | ❌ Adds latency to every scan |
| **Cost Control** | ✅ Explicit, user-initiated | ⚠️ May incur unexpected API costs |
| **Determinism** | ✅ Base scan always deterministic | ⚠️ Results may vary with LLM |
| **Offline Capability** | ✅ Core scanner works offline | ❌ Requires connectivity |
| **Separation of Concerns** | ✅ Clean architecture | ⚠️ Tightly coupled |
| **Debugging** | ✅ Clear two-phase debugging | ⚠️ Mixed responsibility |
| **Zero-Dependency Philosophy** | ✅ Preserves single binary | ❌ Requires LLM integration |

### Two-Phase Architecture

```
Phase 1: Rule-Based Scan (Existing)
┌────────────────────────────────────────────────────────────────┐
│  stack-analyzer scan /project -o analysis.json                 │
│                                                                │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌────────────┐  │
│  │ File     │──▶│ Rule     │──▶│Component │──▶│ JSON       │  │
│  │ Discovery│   │ Matching │   │ Detectors│   │ Output     │  │
│  └──────────┘   └──────────┘   └──────────┘   └────────────┘  │
│                                                                │
│  Output: Deterministic, fast, complete baseline                │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
Phase 2: LLM Enhancement (Optional)
┌────────────────────────────────────────────────────────────────┐
│  stack-analyzer enhance analysis.json --provider openai        │
│                                                                │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌────────────┐  │
│  │ Load     │──▶│ Identify │──▶│ LLM      │──▶│ Enhanced   │  │
│  │ Analysis │   │ Gaps/    │   │ Processing│  │ JSON       │  │
│  │          │   │ Ambiguity│   │          │   │ Output     │  │
│  └──────────┘   └──────────┘   └──────────┘   └────────────┘  │
│                                                                │
│  Output: Enhanced with insights, recommendations, confidence   │
└────────────────────────────────────────────────────────────────┘
```

### CLI Design

```bash
# Phase 1: Standard scan (unchanged, no LLM)
stack-analyzer scan /project -o analysis.json

# Phase 2: LLM enhancement (separate command)
stack-analyzer enhance analysis.json [options]

# Enhancement options
  --provider <name>       LLM provider (openai, anthropic, ollama, azure)
  --model <name>          Specific model to use
  --api-key <key>         API key (or use env var)
  --use-cases <list>      Specific enhancements (disambiguate,recommend,security)
  --output <file>         Enhanced output file (default: analysis-enhanced.json)
  --dry-run               Show what would be enhanced without calling LLM
  --cost-limit <amount>   Maximum API cost in USD
  --offline               Use local model only (e.g., ollama)

# Examples
stack-analyzer enhance analysis.json --provider openai --use-cases disambiguate,recommend
stack-analyzer enhance analysis.json --provider ollama --model llama3 --offline
stack-analyzer enhance analysis.json --dry-run  # Preview enhancement targets
```

---

## Use Case Categories

### 🎯 High-Impact Use Cases

#### 1. Ambiguous Technology Disambiguation

**Problem:** Multiple technologies share similar patterns, making definitive detection difficult.

**Examples:**
- React vs Preact vs Inferno (all use JSX)
- Vue 2 vs Vue 3 (different composition patterns)
- .NET Framework vs .NET Core vs modern .NET
- Express vs Fastify vs Koa (similar middleware patterns)

**Current Limitation:**
```yaml
# Both match same patterns
tech: react
content:
  - pattern: 'import.*from.*["\']react["\']'
    extensions: [.jsx, .tsx]

tech: preact  
content:
  - pattern: 'import.*from.*["\']preact["\']'
    extensions: [.jsx, .tsx]
```

**LLM Enhancement:**
```
Input Context:
- Files: [App.jsx, hooks/useCustom.js, package.json]
- package.json: {"dependencies": {"react": "^18.0.0", "@preact/compat": "^10.0.0"}}
- Code samples: 3-5 representative files

Prompt:
"Analyze this codebase. Both React and Preact are in dependencies. 
 Determine the PRIMARY framework based on:
 1. Import patterns (react vs preact/compat)
 2. Hook usage patterns
 3. Build configuration
 4. Relative dependency counts"

Output:
{
  "primary": "react",
  "confidence": 0.92,
  "reasoning": "Uses react imports in 45/50 files. @preact/compat likely for migration compatibility layer.",
  "secondary": ["preact"],
  "recommendation": "Consider completing migration to Preact or removing compatibility layer"
}
```

**Implementation:**
- Trigger when confidence < 0.8 or multiple conflicting techs detected
- Pass context bundle: file samples + configs + dependency info
- Update confidence scores based on LLM analysis
- Store reasoning in metadata

---

#### 2. Custom Framework Detection

**Problem:** Organizations use internal frameworks that follow no standard patterns.

**Examples:**
- Company-specific UI libraries built on Material-UI
- Internal authentication wrappers around Passport.js
- Custom ORMs wrapping Sequelize/TypeORM
- Proprietary API frameworks on Express/Fastify

**Current Limitation:**
- No rules exist for internal/proprietary tools
- Detection requires manual rule creation for each org
- Naming conventions vary widely

**LLM Enhancement:**
```
Input Context:
- Directory structure with unusual patterns
- Sample import statements
- Config files with non-standard dependencies
- README/documentation files if available

Prompt:
"Analyze this codebase for custom/internal frameworks:
 1. Identify non-standard naming patterns (@company/*, internal-*)
 2. Determine what standard libraries they wrap
 3. Infer framework purpose from usage patterns
 4. Suggest technology category classification"

Output:
{
  "detected_custom_frameworks": [
    {
      "name": "acme-auth",
      "type": "authentication_framework",
      "wraps": ["passport", "jsonwebtoken"],
      "confidence": 0.88,
      "evidence": [
        "Consistent import pattern: import { Auth } from '@acme/auth'",
        "Wraps passport strategies in src/auth/strategies/",
        "Custom session management in @acme/auth/session"
      ],
      "suggested_rule": {
        "tech": "acme-auth",
        "name": "Acme Authentication Framework",
        "type": "authentication",
        "dependencies": [
          {"type": "npm", "name": "@acme/auth"}
        ]
      }
    }
  ]
}
```

**Value:**
- Automatic detection of organizational patterns
- No manual rule creation needed
- Generates suggested rules for permanent addition

---

#### 3. Semantic Dependency Relationship Analysis

**Problem:** Dependencies exist in isolation; no understanding of architectural patterns or missing critical pieces.

**Examples:**
- Database detected but no connection pooling
- Authentication library but no session storage
- API framework but no validation library
- React but no state management for complex apps

**Current State:**
```
Detected:
- postgresql (from pg dependency)
- express
- passport

Missing Context:
- Is connection pooling configured?
- Where are sessions stored?
- Is this production-ready?
```

**LLM Enhancement:**
```
Input Context:
- All detected dependencies with versions
- Technology stack composition
- Project size indicators (file count, LOC estimate)

Prompt:
"Analyze this dependency stack for completeness:
 Database: PostgreSQL
 Framework: Express
 Auth: Passport
 
 Check for:
 1. Production-readiness gaps
 2. Missing critical dependencies
 3. Architectural anti-patterns
 4. Security concerns"

Output:
{
  "completeness_score": 6.5,
  "critical_gaps": [
    {
      "category": "connection_pooling",
      "severity": "high",
      "issue": "PostgreSQL detected without connection pooling library",
      "recommendation": "Add pg-pool or use connection.poolSize in config",
      "risk": "Database connection exhaustion under load"
    },
    {
      "category": "session_storage", 
      "severity": "medium",
      "issue": "Passport authentication without persistent session store",
      "recommendation": "Add connect-redis or express-session with store",
      "risk": "Sessions lost on server restart, not horizontally scalable"
    }
  ],
  "best_practices": [
    {
      "category": "validation",
      "suggestion": "Consider adding joi or express-validator for input validation",
      "priority": "medium"
    }
  ]
}
```

**Value:**
- Proactive architecture quality assessment
- Security and production-readiness insights
- Gap analysis for missing critical components

---

#### 4. Version Compatibility & Migration Path Analysis

**Problem:** Detecting versions is easy; understanding compatibility, EOL status, and migration paths is hard.

**Current Limitation:**
```
Detected:
- react: 16.8.0
- webpack: 4.46.0  
- babel: 6.26.3
- node: 12.22.0

No insight into:
- Which versions are EOL?
- What are breaking changes between versions?
- What's the optimal upgrade path?
```

**LLM Enhancement:**
```
Input Context:
- All detected technologies with exact versions
- Lock file contents (complete dependency tree)
- Target upgrade versions (optional)

Prompt:
"Analyze this technology stack for version health:
 - React 16.8.0 (pre-hooks support was 16.8+, but old point release)
 - Webpack 4.46.0 (webpack 5 released 2020)
 - Babel 6.26.3 (babel 7+ released 2018)
 - Node 12.22.0 (Node 20 LTS available)
 
 Provide:
 1. EOL/support status for each
 2. Breaking change summary for upgrades
 3. Optimal migration path (order matters)
 4. Risk assessment for each upgrade
 5. Estimated effort"

Output:
{
  "overall_health": {
    "score": 3.5,
    "status": "poor",
    "security_risk": "high",
    "maintenance_burden": "high"
  },
  "component_analysis": [
    {
      "name": "babel",
      "current": "6.26.3",
      "status": "EOL since 2018",
      "latest_stable": "7.23.6",
      "priority": "critical",
      "breaking_changes": [
        "Config format changed (.babelrc → babel.config.js)",
        "Preset names changed (@babel/preset-env)",
        "Some plugins merged/deprecated"
      ],
      "effort": "medium",
      "must_upgrade_before": ["webpack"]
    },
    {
      "name": "webpack", 
      "current": "4.46.0",
      "status": "EOL since 2022",
      "latest_stable": "5.89.0",
      "priority": "high",
      "breaking_changes": [
        "Node.js 10 support dropped",
        "Module federation introduced",
        "Asset modules replace file-loader"
      ],
      "effort": "medium-high",
      "dependencies": ["babel must be 7+"]
    }
  ],
  "migration_path": [
    {
      "step": 1,
      "action": "Upgrade Babel 6 → 7",
      "duration": "2-3 days",
      "risk": "medium",
      "blockers": []
    },
    {
      "step": 2,
      "action": "Upgrade Webpack 4 → 5", 
      "duration": "3-5 days",
      "risk": "medium-high",
      "blockers": ["babel 7+ required"]
    },
    {
      "step": 3,
      "action": "Upgrade React 16.8 → 18",
      "duration": "5-7 days",
      "risk": "high",
      "blockers": ["webpack 5 for optimal tree-shaking"]
    },
    {
      "step": 4,
      "action": "Upgrade Node.js 12 → 20 LTS",
      "duration": "1-2 days",
      "risk": "low",
      "blockers": ["all above completed for CI compatibility"]
    }
  ],
  "total_estimated_effort": "15-20 days",
  "recommended_start_date": "Q1 2026"
}
```

**Value:**
- Technical debt quantification
- Prioritized upgrade roadmap
- Risk-aware migration planning

---

#### 5. Monorepo/Multi-Component Relationship Inference

**Problem:** Components detected independently with no cross-component understanding.

**Current Behavior:**
```
Detected Components:
/frontend: React, TypeScript, Webpack
/backend: Node.js, Express, PostgreSQL  
/shared: TypeScript
/mobile: React Native
/docs: Next.js

No understanding of:
- How components relate to each other
- What architecture pattern is used
- Which components share code
```

**LLM Enhancement:**
```
Input Context:
- Component tree structure
- Technologies detected per component
- Workspace configuration (if any)
- Import/export patterns between directories

Prompt:
"Analyze this multi-component project structure:
 - Frontend: React + TS + Webpack
 - Backend: Node.js + Express + PG
 - Shared: TypeScript definitions
 - Mobile: React Native
 - Docs: Next.js
 
 Determine:
 1. Architecture pattern (monorepo, multi-repo, etc.)
 2. Code sharing strategy
 3. Deployment model
 4. Communication patterns between components"

Output:
{
  "architecture_pattern": "monorepo",
  "pattern_confidence": 0.94,
  "evidence": [
    "Shared workspace with TypeScript project references",
    "/shared used by both frontend and backend",
    "Consistent TypeScript configuration"
  ],
  "component_relationships": [
    {
      "source": "frontend",
      "target": "shared",
      "relationship": "imports_types",
      "strength": "strong"
    },
    {
      "source": "backend", 
      "target": "shared",
      "relationship": "imports_types",
      "strength": "strong"
    },
    {
      "source": "mobile",
      "target": "shared", 
      "relationship": "imports_types",
      "strength": "medium"
    }
  ],
  "architectural_insights": [
    "Backend-for-Frontend (BFF) pattern with shared contracts",
    "Type-safe API contracts via shared TypeScript definitions",
    "Mobile reuses business logic types from shared",
    "Documentation likely generated from API specs"
  ],
  "deployment_model": "independent_services",
  "communication_pattern": "rest_api_with_typed_contracts"
}
```

**Value:**
- Higher-level architectural understanding
- Documentation generation material
- Onboarding insights for new developers

---

#### 6. Configuration Intent Understanding

**Problem:** Config files are parsed for patterns, but the "why" is lost.

**Examples:**
- Why multi-stage Docker build?
- Why specific nginx configuration?
- Why certain environment variables?

**LLM Enhancement:**
```
Input Context:
- Dockerfile (multi-stage build with nginx)
- docker-compose.yml (5 services)
- nginx.conf (reverse proxy config)
- .env.example (environment variables)

Prompt:
"Analyze this infrastructure configuration:
 
 Dockerfile: Multi-stage build (node:18-alpine → nginx:alpine)
 Services: api, worker, redis, postgres, nginx
 Nginx: Reverse proxy with rate limiting
 
 Explain the architectural decisions and intent."

Output:
{
  "architecture_assessment": {
    "type": "microservices_with_api_gateway",
    "maturity": "production_ready",
    "confidence": 0.91
  },
  "key_patterns": [
    {
      "pattern": "multi_stage_docker_build",
      "intent": "Optimize image size and separate build/runtime environments",
      "benefit": "Smaller production images, faster deployments"
    },
    {
      "pattern": "nginx_reverse_proxy",
      "intent": "API gateway with rate limiting and SSL termination",
      "benefit": "Centralized request handling, DDoS protection"
    },
    {
      "pattern": "redis_service",
      "intent": "Caching layer and session storage",
      "benefit": "Reduced database load, faster response times"
    },
    {
      "pattern": "worker_service",
      "intent": "Background job processing (async tasks)",
      "benefit": "Offload heavy processing from API service"
    }
  ],
  "production_readiness": {
    "score": 8.5,
    "strengths": [
      "Health checks configured",
      "Resource limits set",
      "Logging to stdout/stderr",
      "Graceful shutdown handling"
    ],
    "improvements": [
      "Consider adding monitoring/metrics (Prometheus)",
      "Add backup strategy for postgres",
      "Document disaster recovery procedures"
    ]
  }
}
```

**Value:**
- Automatic architecture documentation
- Intent preservation for future maintainers
- Best practice validation

---

#### 7. Technology Stack Health Assessment

**Problem:** Detection tells you WHAT is there, not HOW GOOD it is.

**Current Output:**
```json
{
  "techs": ["angular", "rxjs", "typescript", "webpack"],
  "versions": {
    "angular": "8.2.14",
    "rxjs": "5.5.12",
    "typescript": "3.4.5",
    "webpack": "4.46.0"
  }
}
```

**LLM Enhancement:**
```
Input Context:
- All detected technologies with versions
- Dependency tree depth
- File count and project size
- Last git commit date (if available)

Prompt:
"Assess the health of this technology stack:
 - Angular 8.2.14 (current: Angular 17)
 - RxJS 5.5.12 (current: RxJS 7.8)
 - TypeScript 3.4.5 (current: TS 5.3)
 - Webpack 4.46.0 (current: Webpack 5.89)
 
 Provide overall health score and actionable recommendations."

Output:
{
  "overall_health": {
    "score": 4.0,
    "grade": "D",
    "status": "poor",
    "requires_attention": true
  },
  "risk_assessment": {
    "security_risk": "HIGH",
    "reason": "Multiple EOL components with known vulnerabilities",
    "cves_potential": "20+ known CVEs in Angular 8"
  },
  "maintenance_burden": {
    "level": "HIGH", 
    "reason": "9+ major versions behind on Angular",
    "estimated_tech_debt": "6-12 developer-months"
  },
  "component_scores": [
    {
      "name": "angular",
      "score": 3.0,
      "age_behind": "9 major versions",
      "risk": "critical",
      "recommendation": "Immediate upgrade required"
    },
    {
      "name": "rxjs",
      "score": 5.0,
      "age_behind": "2 major versions",
      "risk": "medium",
      "recommendation": "Upgrade when upgrading Angular"
    }
  ],
  "upgrade_priority": [
    {
      "rank": 1,
      "technology": "angular",
      "from": "8.2.14",
      "to": "17.x",
      "priority": "CRITICAL",
      "effort": "large",
      "blockers": []
    },
    {
      "rank": 2,
      "technology": "typescript",
      "from": "3.4.5",
      "to": "5.3.x",
      "priority": "HIGH",
      "effort": "medium",
      "blockers": ["requires Angular 13+"]
    }
  ],
  "estimated_modernization": {
    "duration": "3-6 months",
    "team_size": "2-3 developers",
    "risk_level": "high",
    "recommendation": "Plan migration, consider rewrite for Angular 8→17"
  }
}
```

**Value:**
- Executive-level technology health reporting
- Data-driven modernization planning
- Technical debt quantification

---

#### 8. Natural Language Rule Generation

**Problem:** Creating rules requires understanding YAML format and regex patterns.

**LLM Enhancement:**
```
User Input (Natural Language):
"We use an internal framework called 'AcmeReact' that wraps React with our 
 design system. Files use .acme.tsx extension and always import from 
 @acme/components. It should be detected as a UI framework."

LLM Generated Rule:
```yaml
tech: acmereact
name: Acme React Framework
type: ui_framework
description: Internal design system wrapper around React
extensions: [.acme.tsx]
content:
  - pattern: 'import\s+.*from\s+["\']@acme/components["\']'
    extensions: [.tsx, .ts, .jsx, .js]
dependencies:
  - type: npm
    name: "@acme/components"
    example: "@acme/components"
  - type: npm
    name: /^@acme\/react-.*/
    example: "@acme/react-utils"
is_component: false
properties:
  internal: true
  wrapper_for: react
```

Validation Output:
```json
{
  "rule_generated": true,
  "confidence": 0.89,
  "warnings": [
    "Consider adding content pattern for .acme.tsx to validate actual usage"
  ],
  "test_cases_suggested": [
    {
      "should_match": "src/components/Button.acme.tsx with @acme/components import",
      "should_not_match": "src/Button.tsx without @acme imports"
    }
  ]
}
```
```

**Value:**
- Democratize rule creation (non-technical users)
- Rapid prototyping of detection rules
- Automatic validation and test case generation

---

#### 9. Context-Aware False Positive Reduction

**Problem:** Content patterns sometimes match test fixtures, examples, or documentation.

**Examples:**
- Test fixtures containing mock React code
- Documentation examples with sample imports
- Vendored/copied code not actually used

**Current Behavior:**
```
File: tests/fixtures/mock-react-component.js
Content: import React from 'react'

Detection: React ✓ (FALSE POSITIVE - it's a test fixture)
```

**LLM Enhancement:**
```
Input Context:
- File path: tests/fixtures/mock-react-component.js
- File content: [full content]
- Surrounding directory structure
- package.json location (if any)

Prompt:
"This file matches React patterns, but evaluate if it's actual usage:
 Path: tests/fixtures/mock-react-component.js
 
 Factors to consider:
 1. Is this a test fixture/mock?
 2. Is there a nearby package.json?
 3. Is code commented out or dormant?
 4. Context from surrounding files?"

Output:
{
  "is_actual_usage": false,
  "confidence": 0.95,
  "reasoning": "File is in test fixtures directory, used for testing purposes only",
  "recommendation": "downgrade_confidence",
  "confidence_adjustment": {
    "before": 0.95,
    "after": 0.15,
    "reason": "test_fixture_context"
  }
}
```

**Value:**
- Cleaner detection results
- Reduced noise in reports
- More accurate technology counts

---

#### 10. Intelligent Ecosystem Recommendations

**Problem:** Detection is passive; doesn't suggest what's missing.

**LLM Enhancement:**
```
Input Context:
- Detected: React, TypeScript, Webpack
- Project size: 50+ components
- No testing framework detected
- No linting tools detected

Prompt:
"This is a React + TypeScript project with 50+ components.
 Suggest missing ecosystem tools that would improve quality."

Output:
{
  "missing_critical_tools": [
    {
      "category": "testing",
      "severity": "high",
      "issue": "No testing framework detected",
      "recommendations": [
        {
          "tool": "jest",
          "reason": "Industry standard for React testing",
          "setup_effort": "low",
          "dependencies": ["@testing-library/react", "@testing-library/jest-dom"]
        },
        {
          "tool": "vitest",
          "reason": "Modern, faster alternative to Jest",
          "setup_effort": "low",
          "dependencies": ["@testing-library/react", "vitest"]
        }
      ]
    },
    {
      "category": "code_quality",
      "severity": "medium",
      "issue": "No ESLint detected for TypeScript project",
      "recommendations": [
        {
          "tool": "eslint",
          "reason": "Catch bugs and enforce standards",
          "setup_effort": "low",
          "dependencies": ["@typescript-eslint/eslint-plugin", "@typescript-eslint/parser"]
        }
      ]
    },
    {
      "category": "state_management",
      "severity": "medium",
      "issue": "Large component count (50+) without state management library",
      "recommendations": [
        {
          "tool": "zustand",
          "reason": "Simple, modern state management",
          "setup_effort": "low"
        },
        {
          "tool": "redux-toolkit",
          "reason": "Comprehensive state management for complex apps",
          "setup_effort": "medium"
        }
      ]
    }
  ],
  "nice_to_have": [
    {
      "category": "styling",
      "suggestion": "Consider CSS framework: TailwindCSS or styled-components",
      "benefit": "Consistent styling system"
    },
    {
      "category": "documentation",
      "suggestion": "Consider Storybook for component documentation",
      "benefit": "Visual component library and testing"
    }
  ]
}
```

**Value:**
- Proactive quality improvement suggestions
- Best practice guidance
- Reduced setup time for common patterns

---

#### 19. Architecture Review and Pattern Analysis

**Problem:** Tech stack detection tells you WHAT technologies exist, but not HOW they're organized or if the architecture follows good practices.

**Current Behavior:**
```
Detected Technologies:
- React, TypeScript, Redux, React Router
- Node.js, Express, PostgreSQL, Redis
- Docker, Kubernetes, nginx

No understanding of:
- Is this a well-structured monolith or spaghetti code?
- Are there architectural anti-patterns?
- Does the structure match claimed patterns (e.g., "clean architecture")?
- Are services properly decoupled?
```

**LLM Enhancement:**
```
Input Context:
- Full scan output JSON with:
  - Detected technologies and versions
  - Component hierarchy (frontend/backend/shared)
  - Dependency graph between components
  - Docker/Kubernetes configurations
  - File structure summary
  - Import patterns between modules

Prompt:
"Analyze this project's architecture based on the scan results:

 Detected Stack:
 - Frontend: React 18, TypeScript, Redux Toolkit, React Router
 - Backend: Node.js 20, Express, PostgreSQL, Redis
 - Infrastructure: Docker, nginx, Kubernetes
 
 Component Structure:
 - /frontend (imports from /shared)
 - /backend (imports from /shared)
 - /shared (pure TypeScript, no external imports)
 
 Docker Compose Services:
 - frontend → backend (API calls)
 - backend → postgres, redis
 - nginx → frontend, backend (reverse proxy)
 
 Identify:
 1. Architectural pattern(s) in use
 2. Anti-patterns or red flags
 3. Coupling assessment
 4. Compliance with best practices
 5. Improvement recommendations"

Output:
{
  "detected_patterns": [
    {
      "pattern": "layered_architecture",
      "confidence": 0.87,
      "evidence": [
        "Separate frontend/backend directories",
        "Shared module for cross-cutting concerns",
        "Clear API boundary via Express routes"
      ]
    },
    {
      "pattern": "api_gateway",
      "confidence": 0.92,
      "evidence": [
        "nginx reverse proxy configuration",
        "Route-based traffic splitting"
      ]
    },
    {
      "pattern": "cqrs_partial",
      "confidence": 0.65,
      "evidence": [
        "Redis suggests query caching",
        "Some read/write separation visible"
      ]
    }
  ],
  "anti_patterns_detected": [
    {
      "anti_pattern": "shared_database",
      "severity": "medium",
      "description": "Multiple services may directly access PostgreSQL",
      "recommendation": "Consider service-specific databases or API-based data access"
    },
    {
      "anti_pattern": "missing_circuit_breaker",
      "severity": "low",
      "description": "No resilience patterns detected (Hystrix, Polly, etc.)",
      "recommendation": "Add circuit breaker for external service calls"
    }
  ],
  "coupling_analysis": {
    "overall_score": 7.2,
    "grade": "B",
    "component_coupling": [
      {
        "source": "frontend",
        "target": "backend",
        "coupling_type": "api_contract",
        "strength": "loose",
        "assessment": "healthy"
      },
      {
        "source": "frontend",
        "target": "shared",
        "coupling_type": "type_imports",
        "strength": "medium",
        "assessment": "acceptable"
      },
      {
        "source": "backend",
        "target": "shared",
        "coupling_type": "type_imports",
        "strength": "medium",
        "assessment": "acceptable"
      }
    ],
    "recommendations": [
      "Consider generating API types from OpenAPI spec instead of shared module"
    ]
  },
  "compliance_check": {
    "twelve_factor_app": {
      "score": 8,
      "passing": ["config_in_env", "stateless_processes", "port_binding", "logs_as_streams"],
      "failing": ["backing_services_not_fully_decoupled"],
      "not_assessed": ["dev_prod_parity", "admin_processes"]
    },
    "security_baseline": {
      "score": 6,
      "concerns": [
        "No secrets management tool detected (Vault, AWS Secrets Manager)",
        "Database credentials may be in environment files",
        "No rate limiting configuration visible"
      ]
    }
  },
  "improvement_roadmap": [
    {
      "priority": 1,
      "area": "security",
      "recommendation": "Add secrets management (HashiCorp Vault or cloud equivalent)",
      "effort": "medium",
      "impact": "high"
    },
    {
      "priority": 2,
      "area": "resilience",
      "recommendation": "Implement circuit breaker pattern for service-to-service calls",
      "effort": "low",
      "impact": "medium"
    },
    {
      "priority": 3,
      "area": "observability",
      "recommendation": "Add distributed tracing (Jaeger/Zipkin)",
      "effort": "medium",
      "impact": "medium"
    }
  ],
  "architecture_summary": {
    "style": "Modular Monolith transitioning to Microservices",
    "maturity": "intermediate",
    "strengths": [
      "Clean separation of frontend/backend",
      "Type-safe contracts via shared module",
      "Infrastructure as code with Docker/K8s"
    ],
    "areas_for_improvement": [
      "Service isolation could be stronger",
      "Missing observability stack",
      "Security hardening needed"
    ]
  }
}
```

**CLI Usage:**
```bash
# Full architecture review
stack-analyzer enhance analysis.json --use-cases architecture

# Specific architecture checks
stack-analyzer enhance analysis.json --use-cases architecture --checks patterns,coupling,compliance

# Check against specific standards
stack-analyzer enhance analysis.json --use-cases architecture --standard twelve-factor
stack-analyzer enhance analysis.json --use-cases architecture --standard clean-architecture
```

##### Taxonomy-Driven Architecture Classification

For improved accuracy and consistency, architecture detection can be driven by a **user-provided taxonomy**. Instead of relying on the LLM to invent patterns, it matches evidence against well-defined criteria.

**Architecture Taxonomy File:**
```yaml
# architecture-taxonomy.yaml
version: "1.0"
styles:
  layered:
    description: "Traditional n-tier with presentation, business, data layers"
    aliases: ["n-tier", "three-tier", "multi-tier"]
    indicators:
      - "Separate directories for controllers/services/repositories"
      - "Unidirectional dependencies (UI → Business → Data)"
      - "No direct UI-to-database access"
      - "Layer-specific naming conventions"
    anti_indicators:
      - "Circular dependencies between layers"
      - "UI components directly querying database"
    typical_structure:
      - "controllers/ or handlers/"
      - "services/ or business/"
      - "repositories/ or data/"
      
  hexagonal:
    description: "Ports and adapters, domain at center"
    aliases: ["ports-and-adapters", "onion", "clean-architecture"]
    indicators:
      - "Domain/core directory with no external dependencies"
      - "Adapters/ports directories"
      - "Dependency inversion (infrastructure depends on domain)"
      - "Interface-based boundaries"
    anti_indicators:
      - "Domain importing infrastructure packages"
      - "External libraries in core/domain"
    typical_structure:
      - "domain/ or core/"
      - "adapters/ or infrastructure/"
      - "ports/ or interfaces/"
      
  microservices:
    description: "Independent deployable services"
    aliases: ["distributed", "service-oriented"]
    indicators:
      - "Multiple Dockerfiles or docker-compose services"
      - "Service-specific package.json/go.mod files"
      - "API gateway or service mesh config"
      - "Per-service databases"
      - "Inter-service communication (REST/gRPC/messaging)"
    anti_indicators:
      - "Shared database across services"
      - "Tight coupling between services"
      - "Synchronous chains > 3 services"
    typical_structure:
      - "services/<service-name>/"
      - "Each service has own Dockerfile"
      
  plugin_architecture:
    description: "Core + extensible plugins"
    aliases: ["modular", "extensible"]
    indicators:
      - "Plugin directory with consistent structure"
      - "Plugin manifest files (plugin.json, manifest.yaml)"
      - "Core exports extension points/hooks"
      - "Dynamic loading mechanisms"
    anti_indicators:
      - "Plugins directly modifying core"
      - "Inter-plugin dependencies"
    typical_structure:
      - "core/ or kernel/"
      - "plugins/ or extensions/"
      
  monolith:
    description: "Single deployable unit"
    aliases: ["traditional", "single-deployment"]
    indicators:
      - "Single Dockerfile or deployment config"
      - "Single package.json/go.mod at root"
      - "Shared database access throughout"
    anti_indicators:
      - "Multiple independent deployment configs"
    variants:
      - name: "modular_monolith"
        description: "Well-organized monolith with clear module boundaries"
        additional_indicators:
          - "Clear module/package boundaries"
          - "Module-level interfaces"
      - name: "big_ball_of_mud"
        description: "Poorly organized, high coupling"
        additional_indicators:
          - "Circular dependencies"
          - "No clear boundaries"
          - "Global state usage"

  event_driven:
    description: "Asynchronous event-based communication"
    aliases: ["reactive", "event-sourcing"]
    indicators:
      - "Message queue config (Kafka, RabbitMQ, SQS)"
      - "Event/message handler directories"
      - "Event schema definitions"
      - "Saga/choreography patterns"
    anti_indicators:
      - "Predominantly synchronous REST calls"
```

**CLI with Taxonomy:**
```bash
# Use custom taxonomy
stack-analyzer enhance analysis.json \
  --use-cases architecture \
  --taxonomy architecture-taxonomy.yaml

# Use built-in taxonomy
stack-analyzer enhance analysis.json \
  --use-cases architecture \
  --taxonomy builtin:standard

# Classify against specific styles only
stack-analyzer enhance analysis.json \
  --use-cases architecture \
  --taxonomy architecture-taxonomy.yaml \
  --styles hexagonal,microservices,layered
```

**Taxonomy-Driven Output:**
```json
{
  "taxonomy_version": "1.0",
  "taxonomy_source": "architecture-taxonomy.yaml",
  "classification_results": [
    {
      "style": "hexagonal",
      "confidence": 0.89,
      "matched_indicators": [
        {
          "indicator": "Domain/core directory with no external dependencies",
          "evidence": "/internal/domain has zero external imports (verified via go.mod)"
        },
        {
          "indicator": "Adapters/ports directories",
          "evidence": "Found /internal/adapters/http, /internal/adapters/postgres"
        },
        {
          "indicator": "Dependency inversion",
          "evidence": "adapters/postgres imports domain.Repository interface"
        }
      ],
      "unmatched_indicators": [],
      "anti_indicators_found": []
    },
    {
      "style": "microservices",
      "confidence": 0.45,
      "matched_indicators": [
        {
          "indicator": "Multiple docker-compose services",
          "evidence": "docker-compose.yml defines 4 services"
        }
      ],
      "unmatched_indicators": [
        "Per-service databases (single postgres instance found)",
        "Service-specific package files (single go.mod at root)"
      ],
      "anti_indicators_found": [
        {
          "anti_indicator": "Shared database across services",
          "evidence": "All services connect to same postgres container"
        }
      ]
    },
    {
      "style": "monolith",
      "variant": "modular_monolith",
      "confidence": 0.78,
      "matched_indicators": [
        {
          "indicator": "Single Dockerfile",
          "evidence": "One Dockerfile at root"
        },
        {
          "indicator": "Single go.mod at root",
          "evidence": "Found /go.mod"
        },
        {
          "indicator": "Clear module boundaries",
          "evidence": "internal/ subdirectories with distinct responsibilities"
        }
      ]
    }
  ],
  "primary_classification": {
    "style": "hexagonal",
    "variant": null,
    "confidence": 0.89,
    "summary": "Hexagonal architecture (ports & adapters) with modular monolith deployment"
  },
  "secondary_patterns": ["modular_monolith"],
  "assessment": {
    "architecture_clarity": "high",
    "pattern_consistency": "good",
    "recommendations": [
      "Current hexagonal implementation is solid",
      "Consider microservices only if scaling requires independent deployments"
    ]
  }
}
```

**Benefits of Taxonomy-Driven Classification:**
- **Consistency**: Same criteria applied across all projects
- **Customizable**: Organization can define their own patterns
- **Explainable**: Clear evidence trail for each classification
- **Auditable**: Taxonomy versioned in git alongside code
- **Reducible LLM hallucination**: LLM matches against defined criteria, not inventing patterns

**Value:**
- Automated architecture documentation
- Pattern compliance validation
- Technical debt identification
- Onboarding material for new team members
- Architecture decision records (ADR) generation
- Pre-audit preparation for compliance reviews

---

## Implementation Strategy

### Phase 1: Foundation (Months 1-2)
**Goal:** Prove LLM value with low-risk, high-impact use case

1. **Ambiguous Technology Disambiguation**
   - Start with React/Preact/Vue confusion
   - Build confidence scoring system
   - A/B test against rule-only results
   - Measure: False positive reduction, user satisfaction

2. **Infrastructure Setup**
   - Design LLM API interface
   - Create context bundler (samples + configs)
   - Build response parser and validator
   - Set up monitoring and cost tracking

**Deliverables:**
- `--llm-enhance` flag (opt-in)
- Confidence score improvements
- Clear cost/benefit metrics

---

### Phase 2: Expansion (Months 3-4)
**Goal:** Add value-added analysis features

1. **Custom Framework Detection**
   - Detect internal/proprietary frameworks
   - Generate suggested rules
   - User feedback loop for rule refinement

2. **Stack Health Assessment**
   - Version health scoring
   - EOL detection
   - Basic recommendations

**Deliverables:**
- Custom framework discovery
- Health score in output JSON
- Upgrade priority recommendations

---

### Phase 3: Advanced Features (Months 5-6)
**Goal:** Differentiation and enterprise value

1. **Semantic Dependency Analysis**
   - Production readiness checks
   - Missing critical dependencies
   - Architecture pattern recognition

2. **Migration Path Planning**
   - Version compatibility analysis
   - Upgrade ordering
   - Effort estimation

**Deliverables:**
- Completeness scoring
- Migration roadmaps
- Technical debt quantification

---

### Phase 4: Ecosystem (Months 7+)
**Goal:** Platform and user experience improvements

1. **Natural Language Rule Generation**
   - Conversational rule creation
   - Interactive refinement
   - Automatic test generation

2. **Intelligent Recommendations**
   - Missing tool suggestions
   - Best practice validation
   - Quality improvement guidance

**Deliverables:**
- Rule generation UI/CLI
- Recommendation engine
- Best practice knowledge base

---

## Where NOT to Use LLM

### ❌ Inappropriate Use Cases

1. **Primary Detection Logic**
   - Don't replace rule-based matching
   - Don't use for deterministic operations
   - Don't use for performance-critical paths

2. **High-Frequency Operations**
   - Not for every file scan
   - Not for dependency parsing
   - Not for version extraction

3. **Well-Defined Patterns**
   - package.json parsing
   - Dockerfile syntax
   - Extension matching
   - Lock file parsing

4. **Real-Time Operations**
   - File system traversal
   - Pattern matching
   - Basic categorization

### ✅ Appropriate Use Cases

1. **Post-Scan Analysis** (once per project)
2. **Ambiguous Cases** (< 5% of detections)
3. **High-Level Insights** (architecture, health)
4. **User-Facing Recommendations** (actionable advice)
5. **Rule Generation** (one-time, human-validated)

---

## Technical Considerations

### Performance Impact

```
Baseline (Rule-Only):
- Large repo (10k files): 2-5 seconds
- Medium repo (1k files): 0.5-1 seconds  
- Small repo (100 files): 0.1-0.3 seconds

With LLM Enhancement (opt-in):
- Large repo: +10-30 seconds (post-scan)
- Medium repo: +5-15 seconds (post-scan)
- Small repo: +3-8 seconds (post-scan)

Note: LLM calls are async, don't block primary scan
```

### Cost Management

**Strategies:**
1. **Caching:** Cache LLM responses per project hash
2. **Batching:** Combine multiple questions per API call
3. **Selective:** Only trigger for ambiguous cases
4. **Local Models:** Support local LLM deployment for enterprises

**Cost Estimation:**
```
Per-project LLM cost (GPT-4):
- Small project: $0.02-0.05
- Medium project: $0.10-0.20
- Large project: $0.30-0.50

Annual cost (1000 repos, monthly scans):
- $2,400-6,000/year with caching
- ~70% reduction with local models
```

### Privacy & Security

**Considerations:**
1. **Data Leakage:** Never send proprietary code to external APIs by default
2. **Opt-In:** LLM features strictly opt-in
3. **Local Option:** Support air-gapped deployments
4. **Anonymization:** Strip sensitive strings before LLM analysis
5. **Audit Trail:** Log what was sent to LLM for compliance

**Implementation:**
- `--llm-provider local` for on-premise models
- `--llm-anonymize` to strip sensitive data
- `--llm-audit-log` for compliance tracking

---

## Success Metrics

### Phase 1 Metrics
- **Accuracy:** False positive reduction by 30%+
- **Coverage:** Handle 90%+ of ambiguous cases
- **Performance:** < 10s overhead for medium repos
- **Cost:** < $0.10 per project scan

### Phase 2-3 Metrics
- **User Value:** 80%+ find recommendations useful
- **Adoption:** 40%+ of users enable LLM features
- **Quality:** Generated rules pass validation 85%+ of time
- **Engagement:** 60%+ of users act on recommendations

### Phase 4 Metrics
- **Rule Creation:** 10x faster than manual
- **Community:** 50+ user-contributed rules/month
- **Retention:** 70%+ continue using LLM features
- **Enterprise:** 20+ enterprise deployments

---

## Alternative Approaches

### Option 1: Hybrid (Recommended)
- Core detection: Rule-based (fast, deterministic)
- Enhancement: LLM (context-aware, flexible)
- Best of both worlds

### Option 2: Full LLM
- All detection via LLM
- ❌ Too slow, too expensive
- ❌ Non-deterministic results
- ❌ Requires API access

### Option 3: ML Classification
- Train custom models for detection
- ❌ Requires training data
- ❌ Model maintenance burden
- ❌ Less flexible than LLM

**Verdict:** Hybrid approach maximizes value while maintaining performance and reliability.

---

## Roadmap Summary

```
Quarter 1 (Months 1-3):
├─ Ambiguous tech disambiguation
├─ Confidence scoring system
└─ A/B testing framework

Quarter 2 (Months 4-6):
├─ Custom framework detection
├─ Stack health assessment
└─ Basic recommendations

Quarter 3 (Months 7-9):
├─ Semantic dependency analysis
├─ Migration path planning
└─ Production readiness checks

Quarter 4 (Months 10-12):
├─ Natural language rule generation
├─ Ecosystem recommendations
└─ Enterprise features (local models, audit)
```

---

## Conclusion

LLM enhancement represents a strategic opportunity to add high-value contextual analysis while maintaining the core strength of fast, rule-based detection. By focusing on ambiguous cases, higher-order insights, and user experience improvements, we can differentiate the Tech Stack Analyzer in the market while staying true to its zero-dependency, single-binary philosophy.

**Key Principles:**
- ✅ LLM enhances, never replaces, rule-based detection
- ✅ Opt-in features with clear value proposition
- ✅ Privacy and cost consciousness built-in
- ✅ Focus on actionable insights, not just detection
- ✅ Enterprise-ready with local deployment options

**Next Steps:**
1. Prototype Phase 1 with single use case
2. Measure impact vs. baseline
3. Gather user feedback
4. Iterate before expanding scope
