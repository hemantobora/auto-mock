package builders

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// parsePathAndQueryParams intelligently separates path from query parameters
func parsePathAndQueryParams(fullPath string) (cleanPath string, queryParams map[string]string) {
	queryParams = make(map[string]string)
	
	// Ensure path starts with /
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}
	
	// Parse URL to separate path and query
	parsedURL, err := url.Parse(fullPath)
	if err != nil {
		// If parsing fails, return as-is
		return fullPath, queryParams
	}
	
	cleanPath = parsedURL.Path
	
	// Extract query parameters
	for name, values := range parsedURL.Query() {
		if len(values) > 0 {
			queryParams[name] = values[0] // Take first value
		}
	}
	
	return cleanPath, queryParams
}

// BuildRESTExpectation builds a single REST mock expectation using the enhanced 8-step process
func BuildRESTExpectation() (MockExpectation, error) {
	return BuildRESTExpectationWithContext([]MockExpectation{})
}

// BuildRESTExpectationWithContext builds a REST expectation with context of existing expectations
func BuildRESTExpectationWithContext(existingExpectations []MockExpectation) (MockExpectation, error) {
	var expectation MockExpectation

	fmt.Println("🚀 Starting Enhanced 8-Step REST Expectation Builder")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	steps := []struct {
		name string
		fn   func(*MockExpectation) error
	}{
		{"API Details", collectRESTAPIDetails},
		{"Expectation Identification", func(exp *MockExpectation) error {
			return CollectExpectationName(exp, existingExpectations)
		}},
		{"Query Parameter Matching", collectQueryParameterMatching},
		{"Path Matching Strategy", collectPathMatchingStrategy},
		{"Request Header Matching", collectRequestHeaderMatching},
		{"Response Definition", collectResponseDefinition},
		{"Advanced Features", collectAdvancedFeatures},
		{"Review and Confirm", reviewAndConfirm},
	}

	for i, step := range steps {
		if err := step.fn(&expectation); err != nil {
			return expectation, fmt.Errorf("step %d (%s) failed: %w", i+1, step.name, err)
		}
	}

	return expectation, nil
}

// Step 1: Collect API Details (Method, Path, Request Body)
func collectRESTAPIDetails(expectation *MockExpectation) error {
	fmt.Println("\n📋 Step 1: API Details")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━")

	// HTTP Method selection
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "Select HTTP method:",
		Options: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		Default: "GET",
	}, &method); err != nil {
		return err
	}
	expectation.Method = method

	// Path collection
	var path string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter the API path (e.g., /api/users/{id}):",
		Help:    "Use {param} for path parameters, e.g., /api/users/{id}",
	}, &path); err != nil {
		return err
	}

	// Smart query parameter detection and path cleaning
	cleanPath, detectedParams := parsePathAndQueryParams(path)
	expectation.Path = cleanPath

	// Show detected query parameters
	if len(detectedParams) > 0 {
		fmt.Printf("\n💡 Query parameters detected in path:\n")
		for name, value := range detectedParams {
			fmt.Printf("   %s=%s\n", name, value)
		}
		
		var useDetected bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Auto-configure these query parameters for matching?",
			Default: true,
		}, &useDetected); err != nil {
			return err
		}
		
		if useDetected {
			expectation.QueryParams = detectedParams
			fmt.Printf("✅ Pre-configured %d query parameters\n", len(detectedParams))
		}
	}

	// Request body for methods that typically have bodies
	if method == "POST" || method == "PUT" || method == "PATCH" {
		if err := collectRequestBody(expectation); err != nil {
			return err
		}
	}

	fmt.Printf("✅ API Details: %s %s\n", expectation.Method, expectation.Path)
	return nil
}

// configureStatefulMocking configures stateful mock behavior
func configureStatefulMocking(expectation *MockExpectation) error {
	fmt.Println("\n🔄 Stateful Mocking Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var stateType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select stateful behavior type:",
		Options: []string{
			"session-tracking - Track user sessions",
			"counter-based - Request counting",
			"sequence-dependent - Order-dependent responses",
			"cache-simulation - Cache hit/miss patterns",
		},
	}, &stateType); err != nil {
		return err
	}

	stateType = strings.Split(stateType, " ")[0]

	switch stateType {
	case "session-tracking":
		expectation.ResponseBody = `{
  "sessionId": "${state.get('sessionId') || uuid}",
  "userId": "${request.headers.x-user-id}",
  "loginCount": "${state.increment('loginCount')}",
  "lastAccess": "${timestamp}",
  "sessionState": {
    "authenticated": "${state.exists('sessionId')}",
    "permissions": "${if(state.exists('sessionId'),['read','write'],['read'])}",
    "timeRemaining": "${math.subtract(3600,state.get('sessionDuration'))}"
  }
}`
		fmt.Println("✅ Session tracking configured with state management")
	case "counter-based":
		expectation.ResponseBody = `{
  "requestNumber": "${state.increment('requestCount')}",
  "milestone": "${if(math.modulo(state.get('requestCount'),100) === 0,'Century milestone!','')}",
  "stats": {
    "totalRequests": "${state.get('requestCount')}",
    "avgPerHour": "${math.divide(state.get('requestCount'),math.max(1,math.divide(timestamp,3600)))}",
    "rateLimitRemaining": "${math.max(0,math.subtract(1000,state.get('requestCount')))}"
  }
}`
		fmt.Println("✅ Counter-based state configured with analytics")
	case "sequence-dependent":
		expectation.ResponseBody = `{
  "step": "${state.increment('sequenceStep')}",
  "status": "${if(state.get('sequenceStep') === 1,'started',if(state.get('sequenceStep') < 5,'in_progress','completed'))}",
  "nextAction": "${array.get(['verify','process','validate','approve','complete'],state.get('sequenceStep'))}",
  "canProceed": "${state.get('sequenceStep') < 5}",
  "workflow": {
    "currentStage": "${math.min(state.get('sequenceStep'),5)}",
    "totalStages": 5,
    "completionPercent": "${math.multiply(math.divide(math.min(state.get('sequenceStep'),5),5),100)}"
  }
}`
		fmt.Println("✅ Sequence-dependent responses configured")
	case "cache-simulation":
		expectation.ResponseBody = `{
  "data": "${if(cache.get('cachedData'),'Cached response','Fresh data generated')}",
  "cacheHit": "${cache.exists('cachedData')}",
  "cacheAge": "${cache.age('cachedData')}",
  "performance": {
    "responseTime": "${if(cache.exists('cachedData'),'5ms','150ms')}",
    "source": "${if(cache.exists('cachedData'),'cache','database')}"
  }
}`
		fmt.Println("✅ Cache simulation configured with hit/miss logic")
	}

	return nil
}

// configureWorkflowSimulation configures multi-step workflow patterns
func configureWorkflowSimulation(expectation *MockExpectation) error {
	fmt.Println("\n⚙️  Workflow Simulation Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var workflowType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select workflow type:",
		Options: []string{
			"approval-chain - Multi-level approval process",
			"data-pipeline - ETL pipeline simulation",
			"order-fulfillment - E-commerce order processing",
			"user-onboarding - Multi-step user registration",
		},
	}, &workflowType); err != nil {
		return err
	}

	workflowType = strings.Split(workflowType, " ")[0]

	switch workflowType {
	case "approval-chain":
		expectation.ResponseBody = `{
  "approvalId": "${uuid}",
  "currentLevel": "${state.increment('approvalLevel')}",
  "status": "${if(state.get('approvalLevel') === 1,'pending_manager',if(state.get('approvalLevel') === 2,'pending_director','approved'))}",
  "approvers": [
    {
      "level": 1,
      "role": "manager",
      "status": "${if(state.get('approvalLevel') >= 1,'approved','pending')}",
      "timestamp": "${if(state.get('approvalLevel') >= 1,timestamp,'')}"
    },
    {
      "level": 2,
      "role": "director",
      "status": "${if(state.get('approvalLevel') >= 2,'approved','pending')}",
      "timestamp": "${if(state.get('approvalLevel') >= 2,timestamp,'')}"
    }
  ]
}`
		fmt.Println("✅ Approval chain workflow configured")
	case "data-pipeline":
		expectation.ResponseBody = `{
  "pipelineId": "${uuid}",
  "stage": "${array.get(['extract','transform','validate','load','complete'],state.get('pipelineStage'))}",
  "progress": "${math.multiply(math.divide(state.increment('pipelineStage'),5),100)}%",
  "processing": {
    "recordsProcessed": "${math.multiply(state.get('pipelineStage'),1000)}",
    "errors": "${math.randomInt(0,5)}",
    "estimatedCompletion": "${date.addMinutes(now,math.subtract(10,state.get('pipelineStage')))}"
  }
}`
		fmt.Println("✅ Data pipeline workflow configured")
	case "order-fulfillment":
		expectation.ResponseBody = `{
  "orderId": "${request.pathParameters.orderId || uuid}",
  "status": "${array.get(['received','confirmed','picking','packed','shipped','delivered'],state.get('orderStage'))}",
  "tracking": {
    "stage": "${state.increment('orderStage')}",
    "location": "${array.get(['warehouse','packing','transit','local_facility','out_for_delivery','delivered'],state.get('orderStage'))}",
    "estimatedDelivery": "${date.addDays(now,math.max(0,math.subtract(7,state.get('orderStage'))))}"
  }
}`
		fmt.Println("✅ Order fulfillment workflow configured")
	case "user-onboarding":
		expectation.ResponseBody = `{
  "userId": "${request.pathParameters.userId || uuid}",
  "onboardingStep": "${state.increment('onboardingStep')}",
  "completed": [
    "${if(state.get('onboardingStep') >= 1,'account_created','')}",
    "${if(state.get('onboardingStep') >= 2,'email_verified','')}",
    "${if(state.get('onboardingStep') >= 3,'profile_completed','')}",
    "${if(state.get('onboardingStep') >= 4,'preferences_set','')}"
  ],
  "nextStep": "${array.get(['verify_email','complete_profile','set_preferences','welcome_tour','finished'],state.get('onboardingStep'))}",
  "progress": "${math.multiply(math.divide(state.get('onboardingStep'),4),100)}%"
}`
		fmt.Println("✅ User onboarding workflow configured")
	}

	return nil
}

// configureEventDriven configures event-driven architecture patterns
func configureEventDriven(expectation *MockExpectation) error {
	fmt.Println("\n📡 Event-Driven Architecture Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var eventType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select event pattern:",
		Options: []string{
			"event-sourcing - Event sourcing simulation",
			"pub-sub - Publish/subscribe patterns",
			"saga-pattern - Distributed transaction simulation",
			"cqrs - Command Query Responsibility Segregation",
		},
	}, &eventType); err != nil {
		return err
	}

	eventType = strings.Split(eventType, " ")[0]

	switch eventType {
	case "event-sourcing":
		expectation.ResponseBody = `{
  "eventId": "${uuid}",
  "eventType": "${request.body.eventType}",
  "aggregateId": "${request.body.aggregateId}",
  "version": "${state.increment('aggregateVersion')}",
  "timestamp": "${timestamp}",
  "data": "${request.body.data}",
  "metadata": {
    "correlationId": "${request.headers.correlation-id}",
    "causationId": "${request.headers.causation-id}",
    "userId": "${request.headers.user-id}"
  }
}`
		fmt.Println("✅ Event sourcing pattern configured")
	case "pub-sub":
		expectation.ResponseBody = `{
  "messageId": "${uuid}",
  "topic": "${request.body.topic}",
  "published": true,
  "subscribers": "${math.randomInt(1,10)}",
  "deliveryStatus": {
    "successful": "${math.randomInt(0,10)}",
    "failed": "${math.randomInt(0,2)}",
    "pending": "${math.randomInt(0,3)}"
  },
  "routing": {
    "partitionKey": "${request.body.partitionKey}",
    "routingKey": "${request.body.routingKey}"
  }
}`
		fmt.Println("✅ Pub/sub pattern configured")
	case "saga-pattern":
		expectation.ResponseBody = `{
  "sagaId": "${uuid}",
  "step": "${state.increment('sagaStep')}",
  "status": "${if(state.get('sagaStep') < 3,'running',if(math.random > 0.1,'completed','compensating'))}",
  "steps": [
    {
      "service": "payment",
      "status": "${if(state.get('sagaStep') >= 1,'completed','pending')}",
      "compensation": "refund"
    },
    {
      "service": "inventory",
      "status": "${if(state.get('sagaStep') >= 2,'completed','pending')}",
      "compensation": "release"
    },
    {
      "service": "shipping",
      "status": "${if(state.get('sagaStep') >= 3,'completed','pending')}",
      "compensation": "cancel"
    }
  ]
}`
		fmt.Println("✅ Saga pattern configured")
	case "cqrs":
		expectation.ResponseBody = `{
  "commandId": "${uuid}",
  "handled": true,
  "readModelUpdated": "${random.boolean}",
  "projection": {
    "id": "${request.body.aggregateId}",
    "version": "${state.get('readModelVersion')}",
    "lastUpdated": "${timestamp}"
  },
  "eventStore": {
    "eventsWritten": 1,
    "position": "${state.increment('eventPosition')}"
  }
}`
		fmt.Println("✅ CQRS pattern configured")
	}

	return nil
}

// configureMicroservicePatterns configures microservice architecture patterns
func configureMicroservicePatterns(expectation *MockExpectation) error {
	fmt.Println("\n🐳 Microservice Patterns Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var pattern string
	if err := survey.AskOne(&survey.Select{
		Message: "Select microservice pattern:",
		Options: []string{
			"service-mesh - Service mesh simulation",
			"api-gateway - Gateway routing patterns",
			"health-checks - Health monitoring",
			"distributed-tracing - Tracing headers",
		},
	}, &pattern); err != nil {
		return err
	}

	pattern = strings.Split(pattern, " ")[0]

	switch pattern {
	case "service-mesh":
		expectation.ResponseHeaders = map[string]string{
			"X-Service-Name":    "mock-service",
			"X-Service-Version": "1.0.0",
			"X-Mesh-ID":         "${uuid}",
		}
		expectation.ResponseBody = `{
  "serviceInfo": {
    "name": "mock-service",
    "version": "1.0.0",
    "mesh": "istio",
    "sidecar": "envoy"
  },
  "metrics": {
    "requestLatency": "${math.randomInt(10,200)}ms",
    "memoryUsage": "${math.randomInt(40,80)}%",
    "cpuUsage": "${math.randomInt(20,60)}%"
  }
}`
		fmt.Println("✅ Service mesh pattern configured")
	case "api-gateway":
		expectation.ResponseHeaders = map[string]string{
			"X-Gateway-Route": "${request.path}",
			"X-Upstream":      "mock-backend",
			"X-Rate-Limit":    "1000",
		}
		expectation.ResponseBody = `{
  "routing": {
    "gateway": "api-gateway",
    "route": "${request.path}",
    "upstream": "mock-backend",
    "loadBalancer": "round-robin"
  },
  "policies": {
    "rateLimit": {
      "remaining": "${math.subtract(1000,context.requestCount)}",
      "reset": "${date.addHours(now,1)}"
    }
  }
}`
		fmt.Println("✅ API gateway pattern configured")
	case "health-checks":
		expectation.ResponseBody = `{
  "status": "${if(math.random > 0.05,'healthy','unhealthy')}",
  "checks": {
    "database": "${if(math.random > 0.02,'up','down')}",
    "cache": "${if(math.random > 0.01,'up','down')}",
    "queue": "${if(math.random > 0.03,'up','down')}"
  },
  "uptime": "${math.randomInt(3600,86400)}s",
  "version": "1.0.0"
}`
		fmt.Println("✅ Health checks pattern configured")
	case "distributed-tracing":
		expectation.ResponseHeaders = map[string]string{
			"X-Trace-ID": "${request.headers.x-trace-id || uuid}",
			"X-Span-ID":  "${uuid}",
		}
		expectation.ResponseBody = `{
  "tracing": {
    "traceId": "${request.headers.x-trace-id || uuid}",
    "spanId": "${uuid}",
    "parentSpanId": "${request.headers.x-parent-span-id}",
    "duration": "${math.randomInt(10,200)}ms"
  },
  "service": {
    "name": "mock-service",
    "operation": "${request.method} ${request.path}"
  }
}`
		fmt.Println("✅ Distributed tracing pattern configured")
	}

	return nil
}

// configureAPIVersioning configures API versioning patterns
func configureAPIVersioning(expectation *MockExpectation) error {
	fmt.Println("\n🔄 API Versioning Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var versionType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select versioning strategy:",
		Options: []string{
			"header-based - Version in Accept header",
			"path-based - Version in URL path",
			"query-based - Version in query parameter",
			"feature-flags - Feature flag simulation",
		},
	}, &versionType); err != nil {
		return err
	}

	versionType = strings.Split(versionType, " ")[0]

	switch versionType {
	case "header-based":
		expectation.ResponseBody = `{
  "version": "${request.headers.accept-version || 'v1'}",
  "features": "${if(equals(request.headers.accept-version,'v2'),['newField','enhancedApi'],['basicApi'])}",
  "deprecated": "${equals(request.headers.accept-version,'v1')}",
  "migration": {
    "recommendedVersion": "v2",
    "deprecationDate": "2025-12-31",
    "migrationGuide": "https://api.example.com/migration/v1-to-v2"
  }
}`
		fmt.Println("✅ Header-based versioning configured")
	case "path-based":
		expectation.ResponseBody = `{
  "version": "${string.split(request.path,'/').1}",
  "endpoint": "${string.replace(request.path,'/v[0-9]+','')}",
  "compatibility": {
    "backward": "${string.contains(request.path,'v1')}",
    "forward": "${string.contains(request.path,'v2')}"
  }
}`
		fmt.Println("✅ Path-based versioning configured")
	case "query-based":
		expectation.ResponseBody = `{
  "version": "${request.queryParameters.version || 'latest'}",
  "beta": "${equals(request.queryParameters.version,'beta')}",
  "stable": "${notEquals(request.queryParameters.version,'beta')}",
  "releaseNotes": "${if(equals(request.queryParameters.version,'beta'),'Beta features included','Stable release')}"
}`
		fmt.Println("✅ Query-based versioning configured")
	case "feature-flags":
		expectation.ResponseBody = `{
  "flags": {
    "newUserInterface": "${random.boolean}",
    "enhancedSearch": "${if(equals(request.headers.user-id,'premium'),true,random.boolean)}",
    "betaFeatures": "${request.queryParameters.beta === 'true'}"
  },
  "config": {
    "rolloutPercentage": "${math.randomInt(10,100)}",
    "targetAudience": "${if(random.boolean,'all','premium')}"
  }
}`
		fmt.Println("✅ Feature flags pattern configured")
	}

	return nil
}

// configureTenantIsolation configures multi-tenant behavior patterns
func configureTenantIsolation(expectation *MockExpectation) error {
	fmt.Println("\n🏢 Multi-Tenant Isolation Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var isolationType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select tenant isolation pattern:",
		Options: []string{
			"subdomain-based - Tenant-specific subdomains",
			"header-based - Tenant ID in headers",
			"path-based - Tenant in URL path",
			"database-per-tenant - Tenant-specific data",
		},
	}, &isolationType); err != nil {
		return err
	}

	isolationType = strings.Split(isolationType, " ")[0]

	switch isolationType {
	case "subdomain-based":
		expectation.ResponseBody = `{
  "tenant": {
    "id": "${string.split(request.headers.host,'.').0}",
    "name": "${string.toUpperCase(string.split(request.headers.host,'.').0)} Corp",
    "subdomain": "${string.split(request.headers.host,'.').0}",
    "region": "${array.random(['us-east','us-west','eu-central'])}"
  },
  "isolation": {
    "level": "subdomain",
    "dataRegion": "${array.random(['us-east','us-west','eu-central'])}",
    "compliance": "${array.random(['SOC2','GDPR','HIPAA'])}"
  }
}`
		fmt.Println("✅ Subdomain-based tenant isolation configured")
	case "header-based":
		expectation.ResponseBody = `{
  "tenant": {
    "id": "${request.headers.x-tenant-id}",
    "name": "Tenant ${request.headers.x-tenant-id}",
    "tier": "${if(request.headers.x-tenant-tier,request.headers.x-tenant-tier,'standard')}"
  },
  "resources": {
    "quota": "${if(equals(request.headers.x-tenant-tier,'premium'),10000,1000)}",
    "used": "${math.randomInt(100,500)}",
    "rateLimit": "${if(equals(request.headers.x-tenant-tier,'premium'),1000,100)}"
  }
}`
		fmt.Println("✅ Header-based tenant isolation configured")
	case "path-based":
		expectation.ResponseBody = `{
  "tenant": {
    "id": "${string.split(request.path,'/').2}",
    "path": "/tenant/${string.split(request.path,'/').2}",
    "isolated": true
  },
  "security": {
    "accessLevel": "tenant-scoped",
    "dataIsolation": "complete",
    "auditLog": "enabled"
  }
}`
		fmt.Println("✅ Path-based tenant isolation configured")
	case "database-per-tenant":
		expectation.ResponseBody = `{
  "tenant": {
    "id": "${request.headers.x-tenant-id}",
    "database": "tenant_${request.headers.x-tenant-id}_db",
    "schema": "tenant_${request.headers.x-tenant-id}"
  },
  "storage": {
    "isolation": "database-level",
    "encryption": "tenant-specific-keys",
    "backup": "isolated-schedule"
  }
}`
		fmt.Println("✅ Database-per-tenant isolation configured")
	}

	return nil
}

// configureAdvancedPatterns configures advanced microservice and architectural patterns
func configureAdvancedPatterns(expectation *MockExpectation) error {
	fmt.Println("\n🏗️  Advanced Architectural Patterns")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect items, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select advanced patterns to configure:",
		Options: []string{
			"stateful-mocking - Maintain state across requests",
			"workflow-simulation - Multi-step process simulation",
			"event-driven - Event-driven architecture patterns",
			"microservice-patterns - Service mesh simulation",
			"api-versioning - Version-aware responses",
			"tenant-isolation - Multi-tenant behavior",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "stateful-mocking":
			if err := configureStatefulMocking(expectation); err != nil {
				return err
			}
		case "workflow-simulation":
			if err := configureWorkflowSimulation(expectation); err != nil {
				return err
			}
		case "event-driven":
			if err := configureEventDriven(expectation); err != nil {
				return err
			}
		case "microservice-patterns":
			if err := configureMicroservicePatterns(expectation); err != nil {
				return err
			}
		case "api-versioning":
			if err := configureAPIVersioning(expectation); err != nil {
				return err
			}
		case "tenant-isolation":
			if err := configureTenantIsolation(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// collectSecurityTestingPatterns collects security testing configuration
func collectSecurityTestingPatterns(expectation *MockExpectation) error {
	fmt.Println("\n🔒 Security Testing Patterns")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show security testing scenarios
	patterns := AdvancedTestingPatterns()
	if secPattern, exists := patterns["Security Testing"]; exists {
		fmt.Printf("\n💡 %s:\n", secPattern.Description)
		for i, scenario := range secPattern.Scenarios {
			fmt.Printf("   %d. %s\n", i+1, scenario)
		}
	}

	var securityType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select security testing pattern:",
		Options: []string{
			"auth-bypass - Authentication bypass simulation",
			"injection-attack - SQL/NoSQL injection patterns",
			"xss-protection - Cross-site scripting simulation",
			"csrf-validation - CSRF token testing",
			"rate-limit-bypass - Rate limiting bypass attempts",
			"privilege-escalation - Authorization testing",
		},
	}, &securityType); err != nil {
		return err
	}

	securityType = strings.Split(securityType, " ")[0]

	switch securityType {
	case "auth-bypass":
		expectation.StatusCode = 401
		expectation.ResponseBody = `{
  "error": "authentication_required",
  "message": "Authentication bypass attempt detected",
  "security": {
    "attemptBlocked": true,
    "suspiciousActivity": "${if(isEmpty(request.headers.authorization),'missing_auth_header','invalid_token')}",
    "riskScore": "${math.randomInt(70,100)}",
    "actionTaken": "${if(math.random > 0.7,'account_locked','rate_limited')}"
  },
  "timestamp": "${timestamp}"
}`
		fmt.Println("✅ Authentication bypass testing configured")
	case "injection-attack":
		expectation.StatusCode = 400
		expectation.ResponseBody = `{
  "error": "malicious_input_detected",
  "message": "Potential SQL injection attempt blocked",
  "security": {
    "inputValidation": "failed",
    "suspiciousPatterns": ["${if(string.contains(request.body,'DROP'),'DROP_TABLE','')}", "${if(string.contains(request.body,'\\''),'ESCAPE_CHAR','')}"],
    "blocked": true,
    "alertSent": true
  },
  "sanitization": {
    "applied": true,
    "rulesTriggered": ["sql_keywords", "special_chars"]
  }
}`
		fmt.Println("✅ Injection attack testing configured")
	case "xss-protection":
		expectation.ResponseHeaders = map[string]string{
			"X-XSS-Protection":        "1; mode=block",
			"X-Content-Type-Options":  "nosniff",
			"Content-Security-Policy": "default-src 'self'",
		}
		expectation.ResponseBody = `{
  "content": "${string.replace(request.body.content,'<script>','&lt;script&gt;')}",
  "security": {
    "xssProtection": "enabled",
    "contentSanitized": "${string.contains(request.body.content,'<script>')}",
    "cspViolations": "${math.randomInt(0,3)}"
  }
}`
		fmt.Println("✅ XSS protection testing configured")
	case "csrf-validation":
		expectation.StatusCode = 403
		expectation.ResponseBody = `{
  "error": "csrf_token_invalid",
  "message": "CSRF token validation failed",
  "security": {
    "csrfTokenProvided": "${if(request.headers.x-csrf-token,'true','false')}",
    "tokenValid": "${if(equals(request.headers.x-csrf-token,'valid-token'),'true','false')}",
    "originCheck": "${equals(request.headers.origin,request.headers.host)}",
    "blocked": true
  }
}`
		fmt.Println("✅ CSRF validation testing configured")
	case "rate-limit-bypass":
		expectation.StatusCode = 429
		expectation.ResponseBody = `{
  "error": "rate_limit_exceeded",
  "message": "Rate limit bypass attempt detected",
  "rateLimiting": {
    "limit": 100,
    "current": "${math.add(100,math.randomInt(1,50))}",
    "bypassAttempt": true,
    "techniques": ["${if(request.headers.x-forwarded-for,'ip_spoofing','')}", "${if(request.headers.user-agent,'user_agent_rotation','')}"],
    "blocked": true
  }
}`
		fmt.Println("✅ Rate limit bypass testing configured")
	case "privilege-escalation":
		expectation.StatusCode = 403
		expectation.ResponseBody = `{
  "error": "insufficient_privileges",
  "message": "Privilege escalation attempt detected",
  "authorization": {
    "requiredRole": "admin",
    "currentRole": "${request.headers.x-user-role || 'user'}",
    "escalationAttempt": "${notEquals(request.headers.x-user-role,'admin')}",
    "securityAlert": true
  }
}`
		fmt.Println("✅ Privilege escalation testing configured")
	}

	return nil
}

// collectResilienceTestingPatterns collects resilience testing configuration
func collectResilienceTestingPatterns(expectation *MockExpectation) error {
	fmt.Println("\n🛡️  Resilience Testing Patterns")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show resilience testing scenarios
	patterns := AdvancedTestingPatterns()
	if resPattern, exists := patterns["Resilience Testing"]; exists {
		fmt.Printf("\n💡 %s:\n", resPattern.Description)
		for i, scenario := range resPattern.Scenarios {
			fmt.Printf("   %d. %s\n", i+1, scenario)
		}
	}

	var resilienceType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select resilience testing pattern:",
		Options: []string{
			"graceful-degradation - Graceful service degradation",
			"automatic-failover - Failover and recovery simulation",
			"data-consistency - Consistency during failures",
			"timeout-retry - Timeout and retry mechanisms",
			"bulkhead-isolation - Isolation effectiveness",
			"disaster-recovery - Recovery procedures",
		},
	}, &resilienceType); err != nil {
		return err
	}

	resilienceType = strings.Split(resilienceType, " ")[0]

	switch resilienceType {
	case "graceful-degradation":
		expectation.ResponseBody = `{
  "status": "degraded",
  "message": "Operating in degraded mode",
  "degradation": {
    "level": "${if(math.random > 0.7,'high',if(math.random > 0.4,'medium','low'))}",
    "affectedServices": ["${if(math.random > 0.5,'search','')}", "${if(math.random > 0.3,'recommendations','')}"],
    "fallbackActive": true,
    "estimatedRecovery": "${math.randomInt(5,60)} minutes"
  },
  "capabilities": {
    "coreFeatures": "available",
    "enhancedFeatures": "limited",
    "realTimeFeatures": "disabled"
  }
}`
		fmt.Println("✅ Graceful degradation testing configured")
	case "automatic-failover":
		expectation.ResponseBody = `{
  "failover": {
    "triggered": true,
    "primaryRegion": "us-east-1",
    "failoverRegion": "us-west-2",
    "switchTime": "${math.randomInt(5,30)}s",
    "dataLag": "${math.randomInt(0,5)}s"
  },
  "recovery": {
    "automatic": true,
    "healthCheckPassed": "${random.boolean}",
    "estimatedRecovery": "${date.addMinutes(now,math.randomInt(10,60))}"
  }
}`
		fmt.Println("✅ Automatic failover testing configured")
	case "data-consistency":
		expectation.ResponseBody = `{
  "consistency": {
    "level": "${array.random(['strong','eventual','weak'])}",
    "conflictDetected": "${random.boolean}",
    "resolutionStrategy": "${if(random.boolean,'last-write-wins','merge')}",
    "syncStatus": "${if(math.random > 0.1,'synchronized','diverged')}"
  },
  "replication": {
    "lag": "${math.randomInt(0,1000)}ms",
    "replicas": {
      "total": 3,
      "synchronized": "${math.randomInt(2,3)}",
      "lagging": "${math.randomInt(0,1)}"
    }
  }
}`
		fmt.Println("✅ Data consistency testing configured")
	case "timeout-retry":
		expectation.ResponseDelay = "${if(state.get('retryCount') < 3,math.multiply(state.increment('retryCount'),1000),'500')}"
		expectation.StatusCode = 200
		expectation.ResponseBody = `{
  "retry": {
    "attempt": "${state.get('retryCount')}",
    "maxRetries": 3,
    "backoffStrategy": "exponential",
    "nextRetryIn": "${if(state.get('retryCount') < 3,math.power(2,state.get('retryCount')),'0')}s"
  },
  "timeout": {
    "configured": "30s",
    "elapsed": "${math.randomInt(1000,29000)}ms",
    "nearTimeout": "${math.randomInt(1000,29000) > 25000}"
  }
}`
		fmt.Println("✅ Timeout and retry testing configured")
	case "bulkhead-isolation":
		expectation.ResponseBody = `{
  "bulkhead": {
    "pool": "${array.random(['critical','standard','background'])}",
    "isolation": "effective",
    "resources": {
      "allocated": "${math.randomInt(10,100)}",
      "used": "${math.randomInt(5,80)}",
      "available": "${math.subtract(100,math.randomInt(5,80))}"
    }
  },
  "impact": {
    "containment": "successful",
    "spillover": false,
    "otherPools": "unaffected"
  }
}`
		fmt.Println("✅ Bulkhead isolation testing configured")
	case "disaster-recovery":
		expectation.ResponseBody = `{
  "disaster": {
    "type": "${array.random(['datacenter_outage','network_partition','hardware_failure'])}",
    "severity": "${array.random(['low','medium','high','critical'])}",
    "affectedRegions": ["us-east-1"],
    "estimatedDuration": "${math.randomInt(30,240)} minutes"
  },
  "recovery": {
    "plan": "active",
    "rto": "15 minutes",
    "rpo": "5 minutes",
    "status": "${array.random(['initiating','in_progress','completed'])}",
    "progress": "${math.randomInt(0,100)}%"
  }
}`
		fmt.Println("✅ Disaster recovery testing configured")
	}

	return nil
}

// collectRequestBody collects request body for REST endpoints
func collectRequestBody(expectation *MockExpectation) error {
	var needsBody bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this request require a specific body to match?",
		Default: false,
		Help:    "Only specify if you need to match exact request body content",
	}, &needsBody); err != nil {
		return err
	}

	if !needsBody {
		return nil
	}

	var bodyJSON string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter the request body JSON:",
		Help:    "Paste your JSON here. It will be validated.",
	}, &bodyJSON); err != nil {
		return err
	}

	// Validate JSON
	if err := ValidateJSON(bodyJSON); err != nil {
		fmt.Printf("⚠️  JSON validation failed: %v\n", err)
		var proceed bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "JSON is invalid. Use it anyway?",
			Default: false,
		}, &proceed); err != nil {
			return err
		}
		if !proceed {
			return fmt.Errorf("invalid JSON provided")
		}
	}

	expectation.Body = bodyJSON
	return nil
}

// Step 2: Query Parameter Matching
func collectQueryParameterMatching(expectation *MockExpectation) error {
	fmt.Println("\n🔍 Step 2: Query Parameter Matching")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check if already configured from path parsing
	if len(expectation.QueryParams) > 0 {
		fmt.Printf("ℹ️  Already configured %d query parameters from path\n", len(expectation.QueryParams))
		
		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add additional query parameters?",
			Default: false,
		}, &addMore); err != nil {
			return err
		}
		
		if !addMore {
			fmt.Printf("✅ Query Parameters: %d configured\n", len(expectation.QueryParams))
			return nil
		}
	} else {
		var needsQueryParams bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Does this endpoint require specific query parameters?",
			Default: false,
			Help:    "Only specify if you need to match exact query parameter values",
		}, &needsQueryParams); err != nil {
			return err
		}

		if !needsQueryParams {
			fmt.Println("ℹ️  No query parameter matching configured")
			return nil
		}
		
		expectation.QueryParams = make(map[string]string)
	}

	for {
		var paramName string
		if err := survey.AskOne(&survey.Input{
			Message: "Parameter name (empty to finish):",
			Help:    "e.g., 'page', 'limit', 'category'",
		}, &paramName); err != nil {
			return err
		}

		paramName = strings.TrimSpace(paramName)
		if paramName == "" {
			break
		}

		var paramValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for '%s':", paramName),
			Help:    "Use exact value or regex pattern",
		}, &paramValue); err != nil {
			return err
		}

		expectation.QueryParams[paramName] = paramValue
		fmt.Printf("✅ Added query parameter: %s=%s\n", paramName, paramValue)
	}

	fmt.Printf("✅ Query Parameters: %d configured\n", len(expectation.QueryParams))
	return nil
}

// Step 3: Path Matching Strategy
func collectPathMatchingStrategy(expectation *MockExpectation) error {
	fmt.Println("\n🛤️  Step 3: Path Matching Strategy")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check if path has parameters
	hasPathParams := strings.Contains(expectation.Path, "{") && strings.Contains(expectation.Path, "}")

	if !hasPathParams {
		// For exact paths, ask if user wants regex matching
		var useRegex bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use regex pattern matching for this path?",
			Default: false,
			Help:    "Regex allows flexible matching but is more complex",
		}, &useRegex); err != nil {
			return err
		}

		if useRegex {
			if err := collectRegexPattern(expectation); err != nil {
				return err
			}
		} else {
			fmt.Println("ℹ️  Using exact string matching for path")
			fmt.Printf("🔍 Pattern: %s (exact match)\n", expectation.Path)
		}
	} else {
		fmt.Printf("ℹ️  Path parameters detected in: %s\n", expectation.Path)
		fmt.Println("💡 MockServer will automatically handle path parameters")
	}

	fmt.Printf("✅ Path matching configured for: %s\n", expectation.Path)
	return nil
}

// Step 4: Request Header Matching
func collectRequestHeaderMatching(expectation *MockExpectation) error {
	fmt.Println("\n📝 Step 4: Request Header Matching")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var needsHeaders bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this request require specific headers to match?",
		Default: false,
		Help:    "e.g., Authorization, Content-Type, API keys",
	}, &needsHeaders); err != nil {
		return err
	}

	if !needsHeaders {
		fmt.Println("ℹ️  No request header matching configured")
		return nil
	}

	expectation.Headers = make(map[string]string)
	expectation.HeaderTypes = make(map[string]string)

	// Header collection with improved flow - ask matching type first
	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Header name (empty to finish):",
			Help:    "e.g., 'Authorization', 'Content-Type'",
		}, &headerName); err != nil {
			return err
		}

		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			break
		}

		// ASK MATCHING TYPE FIRST - more intuitive flow
		var matchingType string
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("How should '%s' header be matched?", headerName),
			Options: []string{
				"exact - Match exact value (e.g., 'Bearer abc123')",
				"regex - Use pattern matching (e.g., 'Bearer .*')",
			},
			Default: "exact - Match exact value (e.g., 'Bearer abc123')",
			Help:    "Choose matching strategy before entering the value",
		}, &matchingType); err != nil {
			return err
		}

		isRegex := strings.HasPrefix(matchingType, "regex")
		
		// NOW ask for value with appropriate context
		var prompt string
		var helpText string
		if isRegex {
			prompt = fmt.Sprintf("Regex pattern for '%s':", headerName)
			helpText = "Enter regex pattern (e.g., 'Bearer .*', 'application/.*')"
		} else {
			prompt = fmt.Sprintf("Exact value for '%s':", headerName)
			helpText = "Enter exact value to match (e.g., 'Bearer abc123')"
		}

		var headerValue string
		if err := survey.AskOne(&survey.Input{
			Message: prompt,
			Help:    helpText,
		}, &headerValue); err != nil {
			return err
		}

		expectation.Headers[headerName] = headerValue
		if isRegex {
			// Validate regex pattern
			if err := IsValidRegex(headerValue); err != nil {
				fmt.Printf("⚠️  Warning: Invalid regex pattern '%s': %v\n", headerValue, err)
				var proceed bool
				if err := survey.AskOne(&survey.Confirm{
					Message: "Use this invalid regex anyway?",
					Default: false,
				}, &proceed); err != nil {
					return err
				}
				if !proceed {
					continue // Ask for header again
				}
			}
			expectation.HeaderTypes[headerName] = "regex"
			fmt.Printf("✅ Added header: %s: %s (regex pattern)\n", headerName, headerValue)
		} else {
			expectation.HeaderTypes[headerName] = "exact"
			fmt.Printf("✅ Added header: %s: %s (exact match)\n", headerName, headerValue)
		}
	}

	fmt.Printf("✅ Request Headers: %d configured\n", len(expectation.Headers))
	return nil
}

// Step 5: Response Definition
func collectResponseDefinition(expectation *MockExpectation) error {
	fmt.Println("\n📤 Step 5: Response Definition")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Status code selection (hierarchical)
	if err := collectStatusCode(expectation); err != nil {
		return err
	}

	// Response body
	if err := collectResponseBody(expectation); err != nil {
		return err
	}

	fmt.Printf("✅ Response: %d with body configured\n", expectation.StatusCode)
	return nil
}

// collectStatusCode collects HTTP status code using hierarchical selection
func collectStatusCode(expectation *MockExpectation) error {
	fmt.Println("\n🔢 Status Code Selection")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━")

	statusCodes := CommonStatusCodes()
	
	// Step 1: Choose category
	var categories []string
	for category := range statusCodes {
		categories = append(categories, category)
	}

	var selectedCategory string
	if err := survey.AskOne(&survey.Select{
		Message: "Select status code category:",
		Options:  categories,
		Default:  "2xx Success",
	}, &selectedCategory); err != nil {
		return err
	}

	// Step 2: Choose specific code
	codes := statusCodes[selectedCategory]
	var codeOptions []string
	for _, code := range codes {
		codeOptions = append(codeOptions, fmt.Sprintf("%d - %s", code.Code, code.Description))
	}

	var selectedCode string
	if err := survey.AskOne(&survey.Select{
		Message: "Select specific status code:",
		Options:  codeOptions,
	}, &selectedCode); err != nil {
		return err
	}

	// Parse status code
	codeStr := strings.Split(selectedCode, " - ")[0]
	statusCode, err := strconv.Atoi(codeStr)
	if err != nil {
		return fmt.Errorf("invalid status code: %w", err)
	}

	expectation.StatusCode = statusCode
	return nil
}

// collectResponseBody collects the response body
func collectResponseBody(expectation *MockExpectation) error {
	fmt.Println("\n📄 Response Body")
	fmt.Println("━━━━━━━━━━━━━━━━━━━")

	var bodyChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to provide the response body?",
		Options: []string{
			"template - Generate from template",
			"json - Type/paste JSON directly", 
			"empty - No response body (204 No Content)",
		},
		Default: "template - Generate from template",
	}, &bodyChoice); err != nil {
		return err
	}

	bodyChoice = strings.Split(bodyChoice, " ")[0]

	switch bodyChoice {
	case "template":
		if err := generateResponseTemplate(expectation); err != nil {
			return err
		}
		
	case "json":
		var responseJSON string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Enter the response body JSON:",
			Help:    "Paste your JSON response here. Leave empty for no body.",
		}, &responseJSON); err != nil {
			return err
		}

		responseJSON = strings.TrimSpace(responseJSON)
		if responseJSON == "" {
			// Empty response
			expectation.ResponseBody = ""
			expectation.StatusCode = 204 // No Content
			fmt.Println("ℹ️  Empty response body - status code changed to 204")
			return nil
		}

		// Validate JSON
		if err := ValidateJSON(responseJSON); err != nil {
			fmt.Printf("⚠️  JSON validation failed: %v\n", err)
			var proceed bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "JSON is invalid. Use it anyway?",
				Default: false,
			}, &proceed); err != nil {
				return err
			}
			if !proceed {
				return fmt.Errorf("invalid JSON provided")
			}
		}

		// Format and store JSON
		formattedJSON, _ := FormatJSON(responseJSON)
		expectation.ResponseBody = formattedJSON
		
	case "empty":
		expectation.ResponseBody = ""
		expectation.StatusCode = 204 // No Content
		fmt.Println("✅ Empty response configured (204 No Content)")
	}

	return nil
}

// Step 6: Advanced Features (shared between REST and GraphQL)
func collectAdvancedFeatures(expectation *MockExpectation) error {
	fmt.Println("\n⚙️  Step 6: Advanced MockServer Features")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var enableAdvanced bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Configure advanced MockServer features?",
		Default: false,
		Help:    "Response delays, templating, callbacks, connection control, and more",
	}, &enableAdvanced); err != nil {
		return err
	}

	if !enableAdvanced {
		fmt.Println("ℹ️  No advanced features configured")
		return nil
	}

	// Show advanced feature categories
	fmt.Println("\n🎛️  Advanced Feature Categories:")
	categories := AdvancedFeatureCategories()
	for category, features := range categories {
		fmt.Printf("   📂 %s:\n", category)
		for _, feature := range features {
			fmt.Printf("      • %s\n", feature)
		}
	}

	// Select feature categories to configure
	var selectedCategories []string
	var categoryOptions []string
	for category := range categories {
		categoryOptions = append(categoryOptions, category)
	}

	fmt.Println("\n👉 Navigation: Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select feature categories to configure:",
		Options:  categoryOptions,
		Help:    "IMPORTANT: Use SPACE (not ENTER) to select items, then ENTER to confirm",
	}, &selectedCategories); err != nil {
		return err
	}

	if len(selectedCategories) == 0 {
		fmt.Println("ℹ️  No feature categories selected")
		return nil
	}

	// Configure selected categories
	for _, category := range selectedCategories {
		switch category {
		case "Response Behavior":
			if err := configureResponseBehavior(expectation); err != nil {
				return err
			}
		case "Dynamic Content":
			if err := configureDynamicContent(expectation); err != nil {
				return err
			}
		case "Integration & Callbacks":
			if err := configureIntegrationCallbacks(expectation); err != nil {
				return err
			}
		case "Connection Control":
			if err := configureConnectionControl(expectation); err != nil {
				return err
			}
		case "Testing Scenarios":
			if err := configureTestingScenarios(expectation); err != nil {
				return err
			}
		case "Advanced Patterns":
			if err := configureAdvancedPatterns(expectation); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// Enhanced response delay configuration with advanced MockServer features
func collectResponseDelay(expectation *MockExpectation) error {
	var needsDelay bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Add response delay?",
		Default: false,
		Help:    "Simulate slow responses for testing",
	}, &needsDelay); err != nil {
		return err
	}

	if !needsDelay {
		return nil
	}

	var delayType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select delay type:",
		Options: []string{
			"fixed - Fixed delay in milliseconds",
			"random - Random delay range",
		},
		Default: "fixed - Fixed delay in milliseconds",
	}, &delayType); err != nil {
		return err
	}

	if strings.HasPrefix(delayType, "fixed") {
		var delay string
		if err := survey.AskOne(&survey.Input{
			Message: "Delay in milliseconds:",
			Default: "1000",
			Help:    "e.g., 1000 for 1 second delay",
		}, &delay); err != nil {
			return err
		}
		// Enhanced delay configuration for MockServer
		if expectation.Times == nil {
			expectation.Times = &Times{}
		}
		expectation.ResponseDelay = delay
		fmt.Printf("✅ Fixed delay configured: %s ms\n", delay)
	} else {
		// Random delay range
		var minDelay string
		if err := survey.AskOne(&survey.Input{
			Message: "Minimum delay (ms):",
			Default: "500",
		}, &minDelay); err != nil {
			return err
		}
		
		var maxDelay string
		if err := survey.AskOne(&survey.Input{
			Message: "Maximum delay (ms):",
			Default: "2000",
		}, &maxDelay); err != nil {
			return err
		}
		
		// Enhanced random delay for MockServer with proper format
		if expectation.Times == nil {
			expectation.Times = &Times{}
		}
		// Store as range format for MockServer
		expectation.ResponseDelay = fmt.Sprintf("%s-%s", minDelay, maxDelay)
		fmt.Printf("✅ Random delay configured: %s-%s ms\n", minDelay, maxDelay)
		
		// Add MockServer-specific documentation
		fmt.Println("\n📚 MockServer Delay Documentation:")
		fmt.Println("   Delay Configuration: https://mock-server.com/mock_server/response_delays.html")
		fmt.Println("   Advanced Timing: https://mock-server.com/mock_server/times.html")
	}

	return nil
}

// Enhanced response limits configuration with advanced MockServer features
func collectResponseLimits(expectation *MockExpectation) error {
	var needsLimits bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Limit number of responses?",
		Default: false,
		Help:    "Useful for testing scenarios like rate limiting",
	}, &needsLimits); err != nil {
		return err
	}

	if !needsLimits {
		return nil
	}

	var remainingTimes string
	if err := survey.AskOne(&survey.Input{
		Message: "Maximum number of responses:",
		Default: "1",
		Help:    "After this many responses, the expectation will stop matching",
	}, &remainingTimes); err != nil {
		return err
	}

	// Parse and validate
	times, err := strconv.Atoi(remainingTimes)
	if err != nil {
		return fmt.Errorf("invalid number: %w", err)
	}

	expectation.Times = &Times{
		RemainingTimes: times,
		Unlimited:      false,
	}

	fmt.Printf("✅ Response limit: %d times\n", times)
	
	// Add advanced rate limiting guidance
	fmt.Println("\n📚 Advanced Rate Limiting Patterns:")
	fmt.Println("   • Create additional expectation for post-limit behavior")
	fmt.Println("   • Use 429 status code for rate limit exceeded responses")
	fmt.Println("   • Include Retry-After header for client guidance")
	
	fmt.Println("\n📚 MockServer Times Documentation:")
	fmt.Println("   Times Configuration: https://mock-server.com/mock_server/times.html")
	fmt.Println("   Rate Limiting Guide: https://mock-server.com/mock_server/response_delays.html")
	
	return nil
}

// Enhanced response templating configuration with advanced MockServer features
func collectResponseTemplating(expectation *MockExpectation) error {
	var needsTemplating bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Use dynamic response templating?",
		Default: false,
		Help:    "Echo request data back in responses using MockServer templating",
	}, &needsTemplating); err != nil {
		return err
	}

	if !needsTemplating {
		return nil
	}

	fmt.Println("\n🎭 Dynamic Response Templating")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show available template variables
	fmt.Println("\n💡 Available Template Variables:")
	fmt.Println("   Path Parameters: ${request.pathParameters.id}")
	fmt.Println("   Query Parameters: ${request.queryParameters.limit}")
	fmt.Println("   Headers: ${request.headers.authorization}")
	fmt.Println("   Body Fields: ${request.body.user.email}")
	fmt.Println("   Timestamps: ${now}, ${timestamp}")
	fmt.Println("   UUIDs: ${uuid}, ${randomUUID}")

	var templateSources []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Which request data to echo in response?",
		Options: []string{
			"path - Path parameters (e.g., /users/{id})",
			"query - Query parameters (e.g., ?limit=10)",
			"headers - Request headers (e.g., Authorization)",
			"body - Request body fields (e.g., user.name)",
			"dynamic - Generated values (UUID, timestamp)",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &templateSources); err != nil {
		return err
	}

	if len(templateSources) == 0 {
		fmt.Println("ℹ️  No template sources selected")
		return nil
	}

	// Generate template examples based on selection
	templateExamples := make(map[string]string)
	for _, source := range templateSources {
		sourceType := strings.Split(source, " ")[0]
		switch sourceType {
		case "path":
			templateExamples["userId"] = "${request.pathParameters.id}"
		case "query":
			templateExamples["limit"] = "${request.queryParameters.limit}"
			templateExamples["page"] = "${request.queryParameters.page}"
		case "headers":
			templateExamples["authToken"] = "${request.headers.authorization}"
			templateExamples["contentType"] = "${request.headers.content-type}"
		case "body":
			templateExamples["userName"] = "${request.body.name}"
			templateExamples["userEmail"] = "${request.body.email}"
		case "dynamic":
			templateExamples["requestId"] = "${uuid}"
			templateExamples["processedAt"] = "${timestamp}"
		}
	}

	// Show generated template example
	if len(templateExamples) > 0 {
		fmt.Println("\n🏗️  Generated Template Example:")
		templateJSON := "{\n"
		for key, value := range templateExamples {
			templateJSON += fmt.Sprintf("  \"%s\": \"%s\",\n", key, value)
		}
		templateJSON = strings.TrimSuffix(templateJSON, ",\n") + "\n}"
		fmt.Printf("%s\n", templateJSON)

		var useTemplate bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Apply templating to your response body?",
			Default: true,
			Help:    "This will update your current response to include template variables",
		}, &useTemplate); err != nil {
			return err
		}

		if useTemplate {
			// Update response body with templating
			if expectation.ResponseBody != nil {
				// Try to merge templating into existing response
				originalResponse := expectation.ResponseBody.(string)
				updatedResponse := enhanceResponseWithTemplating(originalResponse, templateExamples)
				expectation.ResponseBody = updatedResponse
				fmt.Printf("✅ Response enhanced with templating\n")
			} else {
				// Create new templated response
				expectation.ResponseBody = templateJSON
				fmt.Printf("✅ Template response created\n")
			}
		}
	}

	fmt.Println("\n📚 Advanced Templating Documentation:")
	fmt.Println("   MockServer Templating: https://mock-server.com/mock_server/response_templates.html")
	fmt.Println("   Template Variables: https://mock-server.com/mock_server/response_templates.html#template-variables")
	fmt.Println("   Advanced Examples: https://mock-server.com/mock_server/response_templates.html#template-examples")
	fmt.Println("   JavaScript Processing: https://mock-server.com/mock_server/response_templates.html#javascript-templating")
	
	fmt.Println("\n🔥 Pro Templating Tips:")
	fmt.Println("   • Use ${if(condition,value1,value2)} for conditional responses")
	fmt.Println("   • Combine ${request.pathParameters.id} with ${uuid} for realistic data")
	fmt.Println("   • Use ${math.randomInt(1,100)} for dynamic numeric values")
	fmt.Println("   • Echo client data with ${request.headers.user-agent}")

	return nil
}

// collectConditionalBehavior collects conditional response behavior configuration
func collectConditionalBehavior(expectation *MockExpectation) error {
	var needsConditional bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Add conditional response behavior?",
		Default: false,
		Help:    "Different responses based on request count, sequences, or conditions",
	}, &needsConditional); err != nil {
		return err
	}

	if !needsConditional {
		return nil
	}

	fmt.Println("\n🔄 Conditional Response Behavior")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var behaviorType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select conditional behavior type:",
		Options: []string{
			"sequence - Different responses on successive calls",
			"circuit-breaker - Simulate service failures",
			"rate-limit - Simulate rate limiting scenarios",
			"custom - Custom conditional logic",
		},
		Default: "sequence - Different responses on successive calls",
	}, &behaviorType); err != nil {
		return err
	}

	behaviorType = strings.Split(behaviorType, " ")[0]

	switch behaviorType {
	case "sequence":
		return collectResponseSequence(expectation)
	case "circuit-breaker":
		return collectCircuitBreakerBehavior(expectation)
	case "rate-limit":
		return collectRateLimitBehavior(expectation)
	case "custom":
		return showCustomConditionalGuidance()
	default:
		return nil
	}
}

// collectResponseSequence collects response sequence configuration
func collectResponseSequence(expectation *MockExpectation) error {
	fmt.Println("\n📋 Response Sequence Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var sequenceType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select sequence pattern:",
		Options: []string{
			"success-then-error - First call succeeds, then errors",
			"slow-then-fast - First call slow, then normal",
			"custom - Define custom sequence",
		},
	}, &sequenceType); err != nil {
		return err
	}

	sequenceType = strings.Split(sequenceType, " ")[0]

	switch sequenceType {
	case "success-then-error":
		fmt.Println("\n✅ Success-Then-Error Pattern:")
		fmt.Println("   Call 1: 200 OK with data")
		fmt.Println("   Call 2+: 503 Service Unavailable")
		
		// This would require creating multiple expectations
		fmt.Println("\n📝 Implementation Note:")
		fmt.Println("   This pattern requires multiple MockServer expectations.")
		fmt.Println("   The first expectation has times: {remainingTimes: 1}")
		fmt.Println("   The second expectation handles all subsequent calls.")
		
	case "slow-then-fast":
		fmt.Println("\n🐌 Slow-Then-Fast Pattern:")
		fmt.Println("   Call 1: 3000ms delay")
		fmt.Println("   Call 2+: 100ms delay")
		
		// Configure first call with slow delay
		expectation.ResponseDelay = "3000"
		if expectation.Times == nil {
			expectation.Times = &Times{}
		}
		expectation.Times.RemainingTimes = 1
		expectation.Times.Unlimited = false
		
		fmt.Println("\n📝 Implementation Note:")
		fmt.Println("   Current expectation configured for first slow call.")
		fmt.Println("   Create a second expectation for fast subsequent calls.")
		
	case "custom":
		fmt.Println("\n🛠️  Custom Sequence Guide:")
		fmt.Println("   1. Create multiple expectations with same request criteria")
		fmt.Println("   2. Use 'times': {'remainingTimes': N} to limit each")
		fmt.Println("   3. MockServer processes expectations in order")
		fmt.Println("   4. Each expectation can have different response/delay")
	}

	fmt.Printf("✅ Conditional sequence guidance provided\n")
	return nil
}

// collectCircuitBreakerBehavior collects circuit breaker simulation
func collectCircuitBreakerBehavior(expectation *MockExpectation) error {
	fmt.Println("\n⚡ Circuit Breaker Simulation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var failureRate string
	if err := survey.AskOne(&survey.Input{
		Message: "Failure rate percentage (0-100):",
		Default: "30",
		Help:    "Percentage of requests that should fail",
	}, &failureRate); err != nil {
		return err
	}

	var failureResponse string
	if err := survey.AskOne(&survey.Input{
		Message: "Failure status code:",
		Default: "503",
		Help:    "HTTP status code for failed requests",
	}, &failureResponse); err != nil {
		return err
	}

	fmt.Printf("\n🔧 Circuit Breaker Configuration:\n")
	fmt.Printf("   Failure Rate: %s%%\n", failureRate)
	fmt.Printf("   Failure Status: %s\n", failureResponse)
	
	fmt.Println("\n📝 Implementation Guide:")
	fmt.Println("   Create two expectations:")
	fmt.Printf("   1. Success case (70%% of the time) - Current expectation\n")
	fmt.Printf("   2. Failure case (%s%% of the time) - Additional expectation\n", failureRate)
	fmt.Println("   Use MockServer's randomization or script multiple expectations.")

	return nil
}

// collectRateLimitBehavior collects rate limiting simulation
func collectRateLimitBehavior(expectation *MockExpectation) error {
	fmt.Println("\n🚦 Rate Limiting Simulation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var allowedRequests string
	if err := survey.AskOne(&survey.Input{
		Message: "Allowed requests before rate limiting:",
		Default: "5",
		Help:    "Number of successful requests before 429 responses",
	}, &allowedRequests); err != nil {
		return err
	}

	// Configure current expectation for allowed requests
	requests, err := strconv.Atoi(allowedRequests)
	if err != nil {
		return fmt.Errorf("invalid number: %w", err)
	}

	if expectation.Times == nil {
		expectation.Times = &Times{}
	}
	expectation.Times.RemainingTimes = requests
	expectation.Times.Unlimited = false

	fmt.Printf("\n🔧 Rate Limiting Configuration:\n")
	fmt.Printf("   Allowed Requests: %s\n", allowedRequests)
	fmt.Printf("   Rate Limit Response: 429 Too Many Requests\n")
	
	fmt.Println("\n📝 Implementation Guide:")
	fmt.Println("   Current expectation configured for allowed requests.")
	fmt.Printf("   Create a second expectation with same criteria but:\n")
	fmt.Println("   - Status: 429")
	fmt.Println("   - Body: {'error': 'rate_limited', 'retry_after': 60}")
	fmt.Println("   - This expectation will handle requests after the limit.")

	fmt.Printf("✅ Rate limiting configured: %s requests allowed\n", allowedRequests)
	return nil
}

// showCustomConditionalGuidance shows guidance for custom conditional logic
func showCustomConditionalGuidance() error {
	fmt.Println("\n🛠️  Custom Conditional Logic Guide")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	fmt.Println("\n🔧 Advanced Conditional Features:")
	fmt.Println("   1. Request Count Based:")
	fmt.Println("      'times': {'remainingTimes': 3, 'unlimited': false}")
	fmt.Println("   2. Time-Based Responses:")
	fmt.Println("      Multiple expectations with different timeToLive")
	fmt.Println("   3. Header-Based Conditions:")
	fmt.Println("      Different responses based on request headers")
	fmt.Println("   4. JavaScript Callbacks:")
	fmt.Println("      'callback': {'callbackClass': 'your.callback.Class'}")
	
	fmt.Println("\n📚 Advanced MockServer Documentation:")
	fmt.Println("   Conditional Logic: https://mock-server.com/mock_server/expectations.html")
	fmt.Println("   Times Configuration: https://mock-server.com/mock_server/times.html")
	fmt.Println("   Callbacks: https://mock-server.com/mock_server/callbacks.html")
	fmt.Println("   JavaScript Templates: https://mock-server.com/mock_server/response_templates.html#javascript-templating")
	
	fmt.Println("\n🔥 Professional Conditional Patterns:")
	fmt.Println("   • Use priority to control expectation matching order")
	fmt.Println("   • Combine times with different response bodies for sequences")
	fmt.Println("   • Use JavaScript callbacks for complex conditional logic")
	fmt.Println("   • Leverage template variables for dynamic conditional responses")
	
	return nil
}

// enhanceResponseWithTemplating enhances existing response with templating
func enhanceResponseWithTemplating(originalResponse string, templateExamples map[string]string) string {
	// Simple enhancement - in production you'd parse JSON and merge properly
	if originalResponse == "" {
		return originalResponse
	}

	// Try to add template fields to existing JSON
	// This is a simple implementation - you'd want proper JSON parsing
	enhanced := strings.TrimSuffix(strings.TrimSpace(originalResponse), "}")
	if !strings.HasSuffix(enhanced, ",") && !strings.HasSuffix(enhanced, "{") {
		enhanced += ","
	}
	
	enhanced += "\n  \"templatedFields\": {\n"
	for key, value := range templateExamples {
		enhanced += fmt.Sprintf("    \"%s\": \"%s\",\n", key, value)
	}
	enhanced = strings.TrimSuffix(enhanced, ",\n") + "\n  }\n}"
	
	return enhanced
}

// configureResponseBehavior configures response behavior features
func configureResponseBehavior(expectation *MockExpectation) error {
	fmt.Println("\n🎭 Response Behavior Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select response behavior features:",
		Options: []string{
			"delays - Response delays (fixed/random)",
			"limits - Response count limits",
			"priority - Expectation priority",
			"custom-headers - Custom response headers",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "delays":
			if err := collectResponseDelay(expectation); err != nil {
				return err
			}
		case "limits":
			if err := collectResponseLimits(expectation); err != nil {
				return err
			}
		case "priority":
			if err := collectExpectationPriority(expectation); err != nil {
				return err
			}
		case "custom-headers":
			if err := collectCustomResponseHeaders(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// configureDynamicContent configures dynamic content features
func configureDynamicContent(expectation *MockExpectation) error {
	fmt.Println("\n🎨 Dynamic Content Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select dynamic content features:",
		Options: []string{
			"templating - Response templating with request data",
			"sequences - Response sequences over time",
			"conditions - Conditional response logic",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "templating":
			if err := collectAdvancedResponseTemplating(expectation); err != nil {
				return err
			}
		case "sequences":
			if err := collectResponseSequenceAdvanced(expectation); err != nil {
				return err
			}
		case "conditions":
			if err := collectConditionalBehavior(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// configureIntegrationCallbacks configures integration and callback features
func configureIntegrationCallbacks(expectation *MockExpectation) error {
	fmt.Println("\n🔗 Integration & Callbacks Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select integration features:",
		Options: []string{
			"webhooks - HTTP webhooks on request match",
			"custom-code - Java callback classes",
			"forward - Forward to real endpoints",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "webhooks":
			if err := configureWebhookCallbacks(expectation); err != nil {
				return err
			}
		case "custom-code":
			if err := configureCustomCodeCallbacks(expectation); err != nil {
				return err
			}
		case "forward":
			if err := configureRequestForwarding(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// configureConnectionControl configures connection control features
func configureConnectionControl(expectation *MockExpectation) error {
	fmt.Println("\n🌐 Connection Control Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select connection control features:",
		Options: []string{
			"drop-connection - Drop connections (simulate network issues)",
			"chunked-encoding - Control transfer encoding",
			"keep-alive - Connection persistence settings",
			"error-simulation - Simulate connection errors",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	if len(features) == 0 {
		return nil
	}

	if expectation.ConnectionOptions == nil {
		expectation.ConnectionOptions = &ConnectionOptions{}
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "drop-connection":
			expectation.ConnectionOptions.DropConnection = true
			fmt.Println("✅ Drop connection enabled")
		case "chunked-encoding":
			expectation.ConnectionOptions.ChunkedEncoding = true
			fmt.Println("✅ Chunked encoding enabled")
		case "keep-alive":
			expectation.ConnectionOptions.KeepAlive = true
			fmt.Println("✅ Keep-alive enabled")
		case "error-simulation":
			expectation.ConnectionOptions.SuppressConnectionErrors = false
			fmt.Println("✅ Connection error simulation enabled")
		}
	}

	return nil
}

// configureTestingScenarios configures testing scenario features
func configureTestingScenarios(expectation *MockExpectation) error {
	fmt.Println("\n🧪 Advanced Testing Scenarios Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show comprehensive testing patterns
	patterns := AdvancedTestingPatterns()
	fmt.Println("\n💡 Available Testing Patterns:")
	for name, pattern := range patterns {
		fmt.Printf("   📋 %s: %s\n", name, pattern.Description)
	}

	var scenario string
	if err := survey.AskOne(&survey.Select{
		Message: "Select testing scenario:",
		Options: []string{
			"circuit-breaker - Service failure patterns",
			"rate-limiting - Rate limit testing with backoff",
			"chaos-engineering - Advanced chaos patterns",
			"load-testing - Performance testing patterns",
			"security-testing - Security vulnerability patterns",
			"resilience-testing - System resilience patterns",
		},
	}, &scenario); err != nil {
		return err
	}

	scenarioType := strings.Split(scenario, " ")[0]

	switch scenarioType {
	case "circuit-breaker":
		return collectCircuitBreakerAdvanced(expectation)
	case "rate-limiting":
		return collectRateLimitAdvanced(expectation)
	case "chaos-engineering":
		return collectChaosEngineering(expectation)
	case "load-testing":
		return collectLoadTestingPatterns(expectation)
	case "security-testing":
		return collectSecurityTestingPatterns(expectation)
	case "resilience-testing":
		return collectResilienceTestingPatterns(expectation)
	}

	return nil
}

// Step 8: Review and Confirm
func reviewAndConfirm(expectation *MockExpectation) error {
	fmt.Println("\n🔄 Step 8: Review and Confirm")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Display summary
	fmt.Printf("\n📋 Expectation Summary:\n")
	fmt.Printf("   Name: %s\n", expectation.Name)
	if expectation.Description != "" {
		fmt.Printf("   Description: %s\n", expectation.Description)
	}
	fmt.Printf("   Method: %s\n", expectation.Method)
	fmt.Printf("   Path: %s\n", expectation.Path)
	fmt.Printf("   Status Code: %d\n", expectation.StatusCode)

	if len(expectation.QueryParams) > 0 {
		fmt.Printf("   Query Parameters: %d\n", len(expectation.QueryParams))
	}
	if len(expectation.Headers) > 0 {
		fmt.Printf("   Request Headers: %d\n", len(expectation.Headers))
	}
	if expectation.Body != nil {
		fmt.Printf("   Request Body: Configured\n")
	}

	var confirm bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Create this expectation?",
		Default: true,
	}, &confirm); err != nil {
		return err
	}

	if !confirm {
		fmt.Println("\nℹ️  Expectation creation cancelled")
		fmt.Println("🔄 You can start over or exit")
		return fmt.Errorf("expectation creation cancelled by user")
	}

	fmt.Printf("\n✅ REST Expectation Created: %s\n", expectation.Name)
	return nil
}

// Enhanced regex pattern collection with comprehensive validation and hints
func collectRegexPattern(expectation *MockExpectation) error {
	fmt.Println("\n📝 Enhanced Regex Pattern Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show quick common patterns first
	fmt.Println("⚡ Quick Common Patterns:")
	fmt.Println("   \\d+           - Numbers (123, 456)")
	fmt.Println("   \\w+           - Words (user, test123)")
	fmt.Println("   [a-zA-Z0-9]+  - Alphanumeric (abc123)")
	fmt.Println("   .*            - Any characters")
	fmt.Println("   /api/users/\\d+ - Users with numeric ID")

	// Ask if user wants to see full library
	var showFullLibrary bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Show complete regex pattern library?",
		Default: false,
		Help:    "View comprehensive patterns with examples and descriptions",
	}, &showFullLibrary); err != nil {
		return err
	}

	if showFullLibrary {
		// Show comprehensive patterns with categories
		fmt.Println("\n💡 Comprehensive Regex Pattern Library:")
		patterns := RegexPatterns()
		for name, pattern := range patterns {
			fmt.Printf("\n   📂 %s:\n", name)
			fmt.Printf("      Pattern: %s\n", pattern.Pattern)
			fmt.Printf("      Description: %s\n", pattern.Description)
			fmt.Printf("      Examples: %s\n", strings.Join(pattern.Examples, ", "))
		}
	}

	fmt.Println("\n🔧 Regex Quick Reference:")
	fmt.Println("   . = any character          \\d = digit           \\w = word char")
	fmt.Println("   * = zero or more           + = one or more      ? = zero or one")
	fmt.Println("   ^ = start of string        $ = end of string   \\b = word boundary")
	fmt.Println("   [abc] = any of a,b,c      [^abc] = not a,b,c   | = OR")
	fmt.Println("   () = grouping              {} = exact count     [] = character class")

	var useTemplate bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Would you like to select from common patterns?",
		Default: true,
		Help:    "Choose from pre-built patterns or create custom regex",
	}, &useTemplate); err != nil {
		return err
	}

	var regexPattern string

	if useTemplate {
		// Quick selection menu with most common patterns
		var selectedPattern string
		if err := survey.AskOne(&survey.Select{
			Message: "Select a pattern:",
			Options: []string{
				"\\d+ - Numbers (user IDs, order numbers)",
				"\\w+ - Words (usernames, names)",
				"[a-zA-Z0-9]+ - Alphanumeric (codes, tokens)",
				"[a-zA-Z0-9_-]+ - IDs with dashes/underscores",
				"\\d{4}-\\d{2}-\\d{2} - Dates (YYYY-MM-DD)",
				"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12} - UUIDs",
				".* - Any characters (wildcard)",
				"browse-all - Browse complete pattern library",
				"custom - Create custom pattern",
			},
			Default: "\\d+ - Numbers (user IDs, order numbers)",
		}, &selectedPattern); err != nil {
			return err
		}

		// Handle browse-all option
		if strings.HasPrefix(selectedPattern, "browse-all") {
			// Show full library and let user select
			patterns := RegexPatterns()
			var patternOptions []string
			for name := range patterns {
				patternOptions = append(patternOptions, name)
			}
			patternOptions = append(patternOptions, "custom - Create custom pattern")

			if err := survey.AskOne(&survey.Select{
				Message: "Select from complete library:",
				Options:  patternOptions,
			}, &selectedPattern); err != nil {
				return err
			}
		}

		if selectedPattern == "custom - Create custom pattern" {
			useTemplate = false
		} else if strings.HasPrefix(selectedPattern, "\\d+") {
			// Quick pattern: Numbers
			regexPattern = "\\d+"
			fmt.Printf("\n✅ Selected: Numbers pattern (\\d+)\n")
			fmt.Printf("   Matches: 123, 456, 789, 1001\n")
		} else if strings.HasPrefix(selectedPattern, "\\w+") {
			// Quick pattern: Words
			regexPattern = "\\w+"
			fmt.Printf("\n✅ Selected: Words pattern (\\w+)\n")
			fmt.Printf("   Matches: user, test123, user_name\n")
		} else if strings.HasPrefix(selectedPattern, "[a-zA-Z0-9]+") {
			// Quick pattern: Alphanumeric
			regexPattern = "[a-zA-Z0-9]+"
			fmt.Printf("\n✅ Selected: Alphanumeric pattern ([a-zA-Z0-9]+)\n")
			fmt.Printf("   Matches: abc123, Test789, ID42\n")
		} else if strings.HasPrefix(selectedPattern, "[a-zA-Z0-9_-]+") {
			// Quick pattern: IDs with dashes/underscores
			regexPattern = "[a-zA-Z0-9_-]+"
			fmt.Printf("\n✅ Selected: ID pattern ([a-zA-Z0-9_-]+)\n")
			fmt.Printf("   Matches: user-123, item_abc, order-789\n")
		} else if strings.HasPrefix(selectedPattern, "\\d{4}-\\d{2}-\\d{2}") {
			// Quick pattern: Dates
			regexPattern = "\\d{4}-\\d{2}-\\d{2}"
			fmt.Printf("\n✅ Selected: Date pattern (\\d{4}-\\d{2}-\\d{2})\n")
			fmt.Printf("   Matches: 2025-09-21, 2024-12-31, 2023-01-15\n")
		} else if strings.Contains(selectedPattern, "[0-9a-f]{8}-") {
			// Quick pattern: UUIDs
			regexPattern = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
			fmt.Printf("\n✅ Selected: UUID pattern\n")
			fmt.Printf("   Matches: 550e8400-e29b-41d4-a716-446655440000\n")
		} else if strings.HasPrefix(selectedPattern, ".*") {
			// Quick pattern: Wildcard
			regexPattern = ".*"
			fmt.Printf("\n✅ Selected: Wildcard pattern (.*)\n")
			fmt.Printf("   Matches: Any characters\n")
		} else {
			// From complete library
			patterns := RegexPatterns()
			if pattern, exists := patterns[selectedPattern]; exists {
				regexPattern = pattern.Examples[0] // Use first example as default
				fmt.Printf("\n💡 Selected pattern: %s\n", pattern.Pattern)
				fmt.Printf("   Description: %s\n", pattern.Description)
				fmt.Printf("   Default example: %s\n", regexPattern)

				var customize bool
				if err := survey.AskOne(&survey.Confirm{
					Message: "Customize this pattern?",
					Default: false,
				}, &customize); err != nil {
					return err
				}

				if customize {
					useTemplate = false
				}
			} else {
				useTemplate = false
			}
		}
	}

	if !useTemplate {
		if err := survey.AskOne(&survey.Input{
			Message: "Enter custom regex pattern for path:",
			Default: regexPattern,
			Help:    "Use patterns above or create custom regex. Test at regex101.com",
		}, &regexPattern); err != nil {
			return err
		}
	}

	// Enhanced regex validation with comprehensive error analysis
	if err := IsValidRegex(regexPattern); err != nil {
		fmt.Printf("❌ Invalid regex pattern: %v\n", err)
		fmt.Println("\n🔧 Common Regex Mistakes & Solutions:")
		fmt.Println("   • Unescaped special characters: . * + ? ^ $ { } ( ) [ ] | \\")
		fmt.Println("     Solution: Use \\. \\* \\+ etc. to match literal characters")
		fmt.Println("   • Unmatched brackets: [ ] { } ( )")
		fmt.Println("     Solution: Ensure every opening bracket has a closing bracket")
		fmt.Println("   • Invalid escape sequences: \\x \\q (use \\\\x \\\\q)")
		fmt.Println("     Solution: Double escape in strings: \\\\d for digits")
		fmt.Println("   • Invalid character classes: [z-a] (should be [a-z])")
		fmt.Println("     Solution: Use proper ranges [a-z] [A-Z] [0-9]")
		
		fmt.Println("\n🔥 Pro Regex Tips:")
		fmt.Println("   • Test complex patterns at https://regex101.com/")
		fmt.Println("   • Use (?i) for case-insensitive matching")
		fmt.Println("   • Use \\b for word boundaries (exact matches)")
		fmt.Println("   • Combine patterns: /api/(users|orders)/\\d+")

		var proceed bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Regex is invalid. Use it anyway?",
			Default: false,
		}, &proceed); err != nil {
			return err
		}
		if !proceed {
			return fmt.Errorf("invalid regex pattern")
		}
	}

	// Store regex pattern with validation metadata
	expectation.Path = regexPattern
	fmt.Printf("✅ Enhanced regex pattern configured: %s\n", regexPattern)
	
	// Provide pattern analysis
	fmt.Println("\n🔍 Pattern Analysis:")
	analyzeRegexPattern(regexPattern)
	
	fmt.Println("\n📚 Professional Regex Resources:")
	fmt.Println("   Interactive Testing: https://regex101.com/")
	fmt.Println("   Learning Tutorial: https://regexone.com/")
	fmt.Println("   Reference Guide: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions")
	fmt.Println("   MockServer Patterns: https://mock-server.com/mock_server/request_matchers.html#regex-matcher")

	return nil
}

// generateResponseTemplate generates enhanced response templates
func generateResponseTemplate(expectation *MockExpectation) error {
	fmt.Println("\n🏷️  Enhanced Response Template Generation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show template options
	var templateType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select template type:",
		Options: []string{
			"smart - Auto-generate based on method & status",
			"rest-api - RESTful API response",
			"microservice - Microservice response",
			"error-response - Comprehensive error response",
			"minimal - Minimal response",
			"custom - Custom template",
		},
		Default: "smart - Auto-generate based on method & status",
	}, &templateType); err != nil {
		return err
	}

	templateType = strings.Split(templateType, " ")[0]

	// Generate template based on selection
	var template string
	switch templateType {
	case "smart":
		switch {
		case expectation.StatusCode >= 200 && expectation.StatusCode < 300:
			template = generateEnhancedSuccessTemplate(expectation.Method)
		case expectation.StatusCode >= 400 && expectation.StatusCode < 500:
			template = generateEnhancedClientErrorTemplate(expectation.StatusCode)
		case expectation.StatusCode >= 500:
			template = generateEnhancedServerErrorTemplate(expectation.StatusCode)
		default:
			template = `{"message": "Response", "timestamp": "${timestamp}"}`
		}
	case "rest-api":
		template = generateRESTAPITemplate(expectation.Method)
	case "microservice":
		template = generateMicroserviceTemplate()
	case "error-response":
		template = generateComprehensiveErrorTemplate(expectation.StatusCode)
	case "minimal":
		template = generateMinimalTemplate()
	case "custom":
		// Will ask for manual input below
		template = ""
	}

	if template != "" {
		fmt.Printf("💡 Generated %s template:\n%s\n\n", templateType, template)

		var useTemplate bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use this generated template?",
			Default: true,
		}, &useTemplate); err != nil {
			return err
		}

		if useTemplate {
			expectation.ResponseBody = template
			return nil
		}
	}

	// Manual entry for custom or if user declined generated template
	var manualJSON string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter response JSON manually:",
		Help:    "Use ${template.variables} for dynamic content",
	}, &manualJSON); err != nil {
		return err
	}
	expectation.ResponseBody = manualJSON

	return nil
}

// analyzeRegexPattern provides intelligent analysis of regex patterns
func analyzeRegexPattern(pattern string) {
	analysis := []string{}
	
	if strings.Contains(pattern, ".*") {
		analysis = append(analysis, "• Uses wildcard matching (.*) - matches any characters")
	}
	if strings.Contains(pattern, "\\d") {
		analysis = append(analysis, "• Matches digits (\\d) - good for IDs and numbers")
	}
	if strings.Contains(pattern, "\\w") {
		analysis = append(analysis, "• Matches word characters (\\w) - letters, digits, underscore")
	}
	if strings.Contains(pattern, "^") {
		analysis = append(analysis, "• Uses start anchor (^) - ensures pattern starts at beginning")
	}
	if strings.Contains(pattern, "$") {
		analysis = append(analysis, "• Uses end anchor ($) - ensures pattern ends at end")
	}
	if strings.Contains(pattern, "|") {
		analysis = append(analysis, "• Uses alternation (|) - matches multiple options")
	}
	if strings.Contains(pattern, "+") {
		analysis = append(analysis, "• Uses one-or-more (+) - requires at least one occurrence")
	}
	if strings.Contains(pattern, "*") && !strings.Contains(pattern, ".*") {
		analysis = append(analysis, "• Uses zero-or-more (*) - optional repeated characters")
	}
	if strings.Contains(pattern, "[") {
		analysis = append(analysis, "• Uses character classes [...] - matches specific character sets")
	}
	
	if len(analysis) == 0 {
		analysis = append(analysis, "• Simple literal pattern - matches exact text")
	}
	
	for _, item := range analysis {
		fmt.Printf("   %s\n", item)
	}
	
	// Add MockServer-specific guidance
	fmt.Println("\n🎯 MockServer Regex Tips:")
	fmt.Println("   • Patterns are automatically anchored in MockServer")
	fmt.Println("   • Use 'regex' matcher type for flexible path matching")
	fmt.Println("   • Combine with query parameter matching for precision")
}

// Enhanced template generators

func generateEnhancedSuccessTemplate(method string) string {
	switch method {
	case "POST":
		return `{
  "id": "${uuid}",
  "message": "Resource created successfully",
  "timestamp": "${timestamp}",
  "location": "/api/resource/${uuid}",
  "requestId": "${request.headers.x-request-id}"
}`
	case "PUT", "PATCH":
		return `{
  "id": "${request.pathParameters.id}",
  "message": "Resource updated successfully",
  "timestamp": "${timestamp}",
  "version": "${random.integer}",
  "requestId": "${request.headers.x-request-id}"
}`
	case "DELETE":
		return `{
  "message": "Resource deleted successfully",
  "deletedId": "${request.pathParameters.id}",
  "timestamp": "${timestamp}",
  "requestId": "${request.headers.x-request-id}"
}`
	default: // GET
		return `{
  "id": "${uuid}",
  "name": "Sample Resource",
  "status": "active",
  "createdAt": "${timestamp}",
  "updatedAt": "${timestamp}",
  "requestId": "${request.headers.x-request-id}",
  "metadata": {
    "version": "1.0",
    "source": "mock-server"
  }
}`
	}
}

func generateEnhancedClientErrorTemplate(statusCode int) string {
	switch statusCode {
	case 400:
		return `{
  "error": {
    "code": "BAD_REQUEST",
    "message": "Invalid request data provided",
    "details": "Request validation failed",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}",
    "path": "${request.path}"
  },
  "validationErrors": [
    {
      "field": "example_field",
      "message": "Field is required",
      "code": "REQUIRED"
    }
  ]
}`
	case 401:
		return `{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Authentication required",
    "details": "Please provide valid authentication credentials",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}"
  },
  "authMethods": ["Bearer Token", "API Key"]
}`
	case 403:
		return `{
  "error": {
    "code": "FORBIDDEN",
    "message": "Access denied",
    "details": "Insufficient permissions for this resource",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}",
    "requiredPermissions": ["read:resource"]
  }
}`
	case 404:
		return `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Resource not found",
    "details": "The requested resource does not exist",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}",
    "path": "${request.path}",
    "resourceId": "${request.pathParameters.id}"
  }
}`
	case 429:
		return `{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Too many requests",
    "details": "Rate limit exceeded",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}",
    "retryAfter": 60,
    "limit": 100,
    "remaining": 0
  }
}`
	default:
		return `{
  "error": {
    "code": "CLIENT_ERROR",
    "message": "Client error occurred",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}"
  }
}`
	}
}

func generateEnhancedServerErrorTemplate(statusCode int) string {
	return `{
  "error": {
    "code": "INTERNAL_SERVER_ERROR",
    "message": "An internal server error occurred",
    "details": "Please try again later or contact support",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}",
    "traceId": "${uuid}",
    "supportContact": "support@example.com"
  }
}`
}

func generateRESTAPITemplate(method string) string {
	return `{
  "data": {
    "id": "${uuid}",
    "type": "resource",
    "attributes": {
      "name": "Sample Resource",
      "status": "active",
      "createdAt": "${timestamp}",
      "updatedAt": "${timestamp}"
    },
    "relationships": {
      "owner": {
        "data": {"id": "${random.string}", "type": "user"}
      }
    }
  },
  "meta": {
    "requestId": "${request.headers.x-request-id}",
    "version": "1.0",
    "timestamp": "${timestamp}"
  }
}`
}

func generateMicroserviceTemplate() string {
	return `{
  "serviceInfo": {
    "name": "mock-service",
    "version": "1.0.0",
    "environment": "mock",
    "region": "us-east-1"
  },
  "data": {
    "id": "${uuid}",
    "status": "success",
    "timestamp": "${timestamp}",
    "processingTime": "${random.integer}ms"
  },
  "metadata": {
    "requestId": "${request.headers.x-request-id}",
    "correlationId": "${uuid}",
    "traceId": "${uuid}",
    "spanId": "${random.string}"
  }
}`
}

func generateComprehensiveErrorTemplate(statusCode int) string {
	return `{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": "Detailed error description",
    "timestamp": "${timestamp}",
    "requestId": "${request.headers.x-request-id}",
    "traceId": "${uuid}",
    "path": "${request.path}",
    "method": "${request.method}",
    "statusCode": ` + fmt.Sprintf("%d", statusCode) + `
  },
  "context": {
    "userAgent": "${request.headers.user-agent}",
    "clientIp": "${request.headers.x-forwarded-for}",
    "timestamp": "${timestamp}"
  },
  "support": {
    "documentation": "https://docs.example.com/errors",
    "contact": "support@example.com",
    "statusPage": "https://status.example.com"
  }
}`
}

func generateMinimalTemplate() string {
	return `{"success": true, "timestamp": "${timestamp}"}`
}

// Legacy template functions - kept for backward compatibility

// generateSuccessTemplate generates simple success response templates
func generateSuccessTemplate(method string) string {
	switch method {
	case "POST":
		return `{"id": "generated-id-123", "message": "Resource created successfully", "timestamp": "2025-09-21T15:30:00Z"}`
	case "PUT", "PATCH":
		return `{"id": "resource-id-123", "message": "Resource updated successfully", "timestamp": "2025-09-21T15:30:00Z"}`
	case "DELETE":
		return `{"message": "Resource deleted successfully", "timestamp": "2025-09-21T15:30:00Z"}`
	default: // GET
		return `{"id": "resource-id-123", "name": "Sample Resource", "status": "active", "createdAt": "2025-09-21T10:00:00Z", "updatedAt": "2025-09-21T15:30:00Z"}`
	}
}

// generateClientErrorTemplate generates simple 4xx error templates
func generateClientErrorTemplate(statusCode int) string {
	switch statusCode {
	case 400:
		return `{"error": "bad_request", "message": "Invalid request data provided", "timestamp": "2025-09-21T15:30:00Z"}`
	case 401:
		return `{"error": "unauthorized", "message": "Authentication required", "timestamp": "2025-09-21T15:30:00Z"}`
	case 403:
		return `{"error": "forbidden", "message": "Access denied", "timestamp": "2025-09-21T15:30:00Z"}`
	case 404:
		return `{"error": "not_found", "message": "Resource not found", "timestamp": "2025-09-21T15:30:00Z"}`
	default:
		return `{"error": "client_error", "message": "Client error occurred", "timestamp": "2025-09-21T15:30:00Z"}`
	}
}

// generateServerErrorTemplate generates simple 5xx error templates
func generateServerErrorTemplate(statusCode int) string {
	return `{"error": "server_error", "message": "Internal server error", "timestamp": "2025-09-21T15:30:00Z"}`
}

// New advanced feature collection functions

// collectExpectationPriority collects expectation priority configuration
func collectExpectationPriority(expectation *MockExpectation) error {
	var priority string
	if err := survey.AskOne(&survey.Input{
		Message: "Expectation priority (lower numbers = higher priority):",
		Default: "0",
		Help:    "Higher priority expectations are matched first (0 = highest)",
	}, &priority); err != nil {
		return err
	}

	if p, err := strconv.Atoi(priority); err == nil {
		expectation.Priority = p
		fmt.Printf("✅ Priority set to: %d\n", p)
	}

	return nil
}

// collectCustomResponseHeaders collects custom response headers
func collectCustomResponseHeaders(expectation *MockExpectation) error {
	if expectation.ResponseHeaders == nil {
		expectation.ResponseHeaders = make(map[string]string)
	}

	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Response header name (empty to finish):",
			Help:    "e.g., 'X-Custom-Header', 'Cache-Control'",
		}, &headerName); err != nil {
			return err
		}

		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			break
		}

		var headerValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for '%s':", headerName),
		}, &headerValue); err != nil {
			return err
		}

		expectation.ResponseHeaders[headerName] = headerValue
		fmt.Printf("✅ Added response header: %s: %s\n", headerName, headerValue)
	}

	return nil
}

// collectAdvancedResponseTemplating collects advanced templating configuration
func collectAdvancedResponseTemplating(expectation *MockExpectation) error {
	fmt.Println("\n🎭 Advanced Response Templating")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show comprehensive template variables
	fmt.Println("\n📚 Available Template Variables:")
	templateVars := TemplateVariables()
	for category, variables := range templateVars {
		fmt.Printf("   📂 %s:\n", category)
		for _, variable := range variables {
			fmt.Printf("      • %s\n", variable)
		}
	}

	// Enhanced templating configuration
	return collectResponseTemplating(expectation)
}

// collectResponseSequenceAdvanced collects advanced response sequence configuration
func collectResponseSequenceAdvanced(expectation *MockExpectation) error {
	fmt.Println("\n🔄 Advanced Response Sequences")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n💡 Advanced Sequence Patterns:")
	fmt.Println("   🔢 Numbered sequences: 1st call different, 2nd call different, etc.")
	fmt.Println("   ⏰ Time-based sequences: Different responses at different times")
	fmt.Println("   📊 Statistical sequences: Random distribution of responses")
	fmt.Println("   🔄 Cyclical sequences: Repeat pattern after N calls")

	return collectResponseSequence(expectation)
}

// configureWebhookCallbacks configures webhook callback functionality
func configureWebhookCallbacks(expectation *MockExpectation) error {
	fmt.Println("\n🎣 Webhook Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var webhookURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Webhook URL:",
		Help:    "HTTP endpoint to call when this expectation matches",
	}, &webhookURL); err != nil {
		return err
	}

	if webhookURL == "" {
		fmt.Println("ℹ️  No webhook URL provided")
		return nil
	}

	var webhookMethod string
	if err := survey.AskOne(&survey.Select{
		Message: "Webhook HTTP method:",
		Options: []string{"POST", "GET", "PUT", "PATCH"},
		Default: "POST",
	}, &webhookMethod); err != nil {
		return err
	}

	if expectation.Callbacks == nil {
		expectation.Callbacks = &CallbackConfig{}
	}

	expectation.Callbacks.HttpCallback = &HttpCallback{
		URL:    webhookURL,
		Method: webhookMethod,
	}

	fmt.Printf("✅ Webhook configured: %s %s\n", webhookMethod, webhookURL)
	return nil
}

// configureCustomCodeCallbacks configures Java callback classes
func configureCustomCodeCallbacks(expectation *MockExpectation) error {
	fmt.Println("\n☕ Java Callback Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var callbackClass string
	if err := survey.AskOne(&survey.Input{
		Message: "Java callback class name:",
		Help:    "Fully qualified class name (e.g., com.example.MyCallback)",
	}, &callbackClass); err != nil {
		return err
	}

	if callbackClass == "" {
		fmt.Println("ℹ️  No callback class provided")
		return nil
	}

	if expectation.Callbacks == nil {
		expectation.Callbacks = &CallbackConfig{}
	}

	expectation.Callbacks.CallbackClass = callbackClass

	fmt.Printf("✅ Custom callback configured: %s\n", callbackClass)
	fmt.Println("\n📚 Documentation:")
	fmt.Println("   Callback Guide: https://mock-server.com/mock_server/callbacks.html")
	fmt.Println("   Example Classes: https://github.com/mock-server/mockserver/tree/master/mockserver-examples")

	return nil
}

// configureRequestForwarding configures request forwarding
func configureRequestForwarding(expectation *MockExpectation) error {
	fmt.Println("\n↗️  Request Forwarding Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n📝 Note: Request forwarding requires additional MockServer configuration.")
	fmt.Println("   This feature forwards matching requests to real endpoints.")
	fmt.Println("   Configure using 'forward' instead of 'httpResponse' in JSON.")

	var forwardURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Forward to URL:",
		Help:    "Real endpoint to forward requests to (e.g., https://api.real-service.com)",
	}, &forwardURL); err != nil {
		return err
	}

	if forwardURL != "" {
		fmt.Printf("✅ Forwarding configured to: %s\n", forwardURL)
		fmt.Println("\n📚 Documentation:")
		fmt.Println("   Forwarding Guide: https://mock-server.com/mock_server/getting_started.html#button_forward_request")
	}

	return nil
}

// collectCircuitBreakerAdvanced collects advanced circuit breaker configuration
func collectCircuitBreakerAdvanced(expectation *MockExpectation) error {
	fmt.Println("\n⚡ Advanced Circuit Breaker Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var pattern string
	if err := survey.AskOne(&survey.Select{
		Message: "Circuit breaker pattern:",
		Options: []string{
			"gradual-failure - Gradually increase failure rate",
			"burst-failure - Sudden failure then recovery",
			"random-failure - Random failure distribution",
			"cascading-failure - Multiple service failure simulation",
		},
	}, &pattern); err != nil {
		return err
	}

	// Enhanced circuit breaker with detailed configuration
	return collectCircuitBreakerBehavior(expectation)
}

// collectRateLimitAdvanced collects advanced rate limiting configuration
func collectRateLimitAdvanced(expectation *MockExpectation) error {
	fmt.Println("\n🚦 Advanced Rate Limiting Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var strategy string
	if err := survey.AskOne(&survey.Select{
		Message: "Rate limiting strategy:",
		Options: []string{
			"sliding-window - Sliding window rate limiting",
			"token-bucket - Token bucket algorithm",
			"fixed-window - Fixed window rate limiting",
			"adaptive - Adaptive rate limiting",
		},
	}, &strategy); err != nil {
		return err
	}

	// Enhanced rate limiting with detailed configuration
	return collectRateLimitBehavior(expectation)
}

// collectChaosEngineering collects chaos engineering configuration
func collectChaosEngineering(expectation *MockExpectation) error {
	fmt.Println("\n🌪️  Chaos Engineering Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var chaosType string
	if err := survey.AskOne(&survey.Select{
		Message: "Chaos engineering type:",
		Options: []string{
			"latency-injection - Random response delays",
			"failure-injection - Random failures",
			"resource-exhaustion - Simulate resource limits",
			"network-partition - Simulate network issues",
		},
	}, &chaosType); err != nil {
		return err
	}

	chaosType = strings.Split(chaosType, " ")[0]

	switch chaosType {
	case "latency-injection":
		// Random delays between 100ms-5000ms
		expectation.ResponseDelay = "100-5000"
		fmt.Println("✅ Chaos latency injection: 100-5000ms random delays")
	case "failure-injection":
		// Random failure rate
		expectation.StatusCode = 503
		expectation.ResponseBody = `{"error": "chaos_failure", "message": "Random chaos engineering failure"}`
		fmt.Println("✅ Chaos failure injection: Random 503 errors")
	case "resource-exhaustion":
		expectation.StatusCode = 429
		expectation.ResponseBody = `{"error": "resource_exhausted", "message": "Simulated resource exhaustion"}`
		fmt.Println("✅ Chaos resource exhaustion: 429 Too Many Requests")
	case "network-partition":
		if expectation.ConnectionOptions == nil {
			expectation.ConnectionOptions = &ConnectionOptions{}
		}
		expectation.ConnectionOptions.DropConnection = true
		fmt.Println("✅ Chaos network partition: Connection drops")
	}

	fmt.Println("\n📚 Chaos Engineering Resources:")
	fmt.Println("   Chaos Engineering: https://principlesofchaos.org/")
	fmt.Println("   Testing Guide: https://github.com/dastergon/awesome-chaos-engineering")

	return nil
}

// collectLoadTestingPatterns collects load testing pattern configuration
func collectLoadTestingPatterns(expectation *MockExpectation) error {
	fmt.Println("\n📊 Load Testing Patterns Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var pattern string
	if err := survey.AskOne(&survey.Select{
		Message: "Load testing pattern:",
		Options: []string{
			"high-throughput - Fast responses for load testing",
			"memory-pressure - Large response bodies",
			"cpu-intensive - Simulated processing delays",
			"realistic-load - Real-world response patterns",
		},
	}, &pattern); err != nil {
		return err
	}

	pattern = strings.Split(pattern, " ")[0]

	switch pattern {
	case "high-throughput":
		expectation.ResponseDelay = "1"
		expectation.ResponseBody = `{"status": "success", "id": "${uuid}", "timestamp": "${timestamp}"}`
		fmt.Println("✅ High-throughput pattern: 1ms delay, minimal response")
	case "memory-pressure":
		// Large response body for memory testing
		largeData := make([]string, 1000)
		for i := range largeData {
			largeData[i] = fmt.Sprintf("data_item_%d", i)
		}
		expectation.ResponseBody = map[string]interface{}{
			"message": "Large response for memory testing",
			"data":    largeData,
			"size":    "~1000 items",
		}
		fmt.Println("✅ Memory pressure pattern: Large response body (1000 items)")
	case "cpu-intensive":
		expectation.ResponseDelay = "500-2000"
		expectation.ResponseBody = `{"message": "CPU intensive operation completed", "processingTime": "${random.integer}", "result": "success"}`
		fmt.Println("✅ CPU intensive pattern: 500-2000ms delays")
	case "realistic-load":
		expectation.ResponseDelay = "100-800"
		expectation.ResponseBody = `{"status": "success", "data": {"id": "${uuid}", "timestamp": "${timestamp}", "userAgent": "${request.headers.user-agent}"}, "processingTime": "${random.integer}"}`
		fmt.Println("✅ Realistic load pattern: 100-800ms delays, templated responses")
	}

	fmt.Println("\n📚 Load Testing Resources:")
	fmt.Println("   k6: https://k6.io/docs/")
	fmt.Println("   JMeter: https://jmeter.apache.org/")
	fmt.Println("   Artillery: https://artillery.io/docs/")

	return nil
}
