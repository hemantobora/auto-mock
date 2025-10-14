package builders

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// BuildRESTExpectation builds a single REST mock expectation using the enhanced 8-step process
func BuildRESTExpectation() (MockExpectation, error) {
	return BuildRESTExpectationWithContext([]MockExpectation{})
}

// BuildRESTExpectationWithContext builds a REST expectation with context of existing expectations
func BuildRESTExpectationWithContext(existingExpectations []MockExpectation) (MockExpectation, error) {
	var expectation MockExpectation
	var mock_configurator MockConfigurator

	fmt.Println("🚀 Starting Enhanced 7-Step REST Expectation Builder")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	steps := []struct {
		name string
		fn   func(step int, exp *MockExpectation) error
	}{
		{"API Details", collectRESTAPIDetails},
		{"Query Parameter Matching", mock_configurator.CollectQueryParameterMatching},
		{"Path Matching Strategy", mock_configurator.CollectPathMatchingStrategy},
		{"Request Header Matching", mock_configurator.CollectRequestHeaderMatching},
		{"Response Definition", collectResponseDefinition},
		{"Advanced Features", mock_configurator.CollectAdvancedFeatures},
		{"Review and Confirm", reviewAndConfirm},
	}

	for i, step := range steps {
		if err := step.fn(i+1, &expectation); err != nil {
			return expectation, &models.ExpectationBuildError{
				ExpectationType: "REST",
				Step:            step.name,
				Cause:           err,
			}
		}
	}

	return expectation, nil
}

// Step 1: Collect API Details (Method, Path, Request Body)
func collectRESTAPIDetails(step int, expectation *MockExpectation) error {
	fmt.Printf("\n📋 Step %d: API Details\n", step)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━")
	var mock_configurator MockConfigurator

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
	cleanPath, detectedParams := mock_configurator.ParsePathAndQueryParams(path)
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
		if err := mock_configurator.CollectRequestBody(expectation, ""); err != nil {
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

// Step 5: Response Definition
func collectResponseDefinition(step int, expectation *MockExpectation) error {
	fmt.Printf("\n📤 Step %d: Response Definition\n", step)
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
		Options: categories,
		Default: "2xx Success",
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
		Options: codeOptions,
	}, &selectedCode); err != nil {
		return err
	}

	// Parse status code
	codeStr := strings.Split(selectedCode, " - ")[0]
	statusCode, err := strconv.Atoi(codeStr)
	if err != nil {
		return &models.InputValidationError{
			InputType: "status code",
			Value:     codeStr,
			Expected:  "valid HTTP status code",
			Cause:     err,
		}
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
		Default: "json - Type/paste JSON directly",
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
				return &models.JSONValidationError{
					Context: "response body",
					Content: responseJSON,
					Cause:   err,
				}
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

// Step 8: Review and Confirm
func reviewAndConfirm(step int, expectation *MockExpectation) error {
	fmt.Printf("\n🔄 Step %d: Review and Confirm\n", step)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Display summary
	fmt.Printf("\n📋 Expectation Summary:\n")
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

	fmt.Printf("\n✅ REST Expectation Created: %s\n", expectation.Description)
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
				Options: patternOptions,
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
			return &models.RegexValidationError{
				Pattern: regexPattern,
				Context: "path matching",
				Cause:   err,
			}
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
