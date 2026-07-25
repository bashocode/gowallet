package examples

import (
	"net/http"

	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/shared/security"
	"github.com/gin-gonic/gin"
)

// SetupCSRFProtection demonstrates how to integrate CSRF protection into your Gin application.
// This example shows the recommended setup for a web application that serves both API and frontend.
func SetupCSRFProtection() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// 1. Add Security Headers (includes CSP, X-Frame-Options, etc.)
	r.Use(middleware.SecurityHeaders())

	// 2. Add Origin Validation (additional CSRF defense layer)
	// Configure allowed origins based on your environment
	allowedOrigins := []string{
		"https://wallet.example.com",       // Production frontend
		"https://admin.wallet.example.com", // Admin panel
		"http://localhost:3000",            // Local development
		"http://localhost:8080",            // Local development alternate
	}
	r.Use(middleware.OriginValidator(allowedOrigins))

	// 3. Add CSRF Middleware (Double Submit Cookie pattern)
	r.Use(middleware.CSRFMiddleware())

	// 4. Add Error Handler (formats CSRF errors as JSON)
	r.Use(middleware.ErrorHandler())

	// CSRF Token Endpoint
	// Frontend should call this on app initialization to obtain the CSRF token
	r.GET("/api/v1/csrf-token", func(c *gin.Context) {
		token, err := c.Cookie("csrf_token")
		if err != nil || token == "" {
			// Generate new token if cookie doesn't exist
			token, err = security.GenerateCSRFToken()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to generate CSRF token",
				})
				return
			}
			security.SetCSRFCookie(c, token)
		}

		c.JSON(http.StatusOK, gin.H{
			"csrf_token": token,
			"message":    "Include this token as X-CSRF-Token header in all state-changing requests",
		})
	})

	// Example: Safe endpoint (GET) - CSRF not required, but cookie is set
	r.GET("/api/v1/balance", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"balance": 1000.50,
		})
	})

	// Example: State-changing endpoint (POST) - CSRF protection enforced
	r.POST("/api/v1/transactions/transfer", func(c *gin.Context) {
		var req struct {
			Receiver string  `json:"receiver" binding:"required"`
			Amount   float64 `json:"amount" binding:"required,gt=0"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		// Process transfer...
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Transfer completed",
			"transaction": gin.H{
				"receiver": req.Receiver,
				"amount":   req.Amount,
			},
		})
	})

	// Example: Other state-changing endpoints
	r.PUT("/api/v1/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Profile updated"})
	})

	r.DELETE("/api/v1/account", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Account deleted"})
	})

	// Example: Login endpoint showcasing CSRF Token Rotation
	r.POST("/api/v1/login", func(c *gin.Context) {
		// Authenticate user credentials...

		// Rotate CSRF Token after successful login to prevent Session Fixation
		newToken, err := security.RotateCSRFToken(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to rotate CSRF token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message":    "Login successful",
			"csrf_token": newToken,
		})
	})

	r.PATCH("/api/v1/settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Settings updated"})
	})

	return r
}

// Frontend Integration Example (JavaScript/TypeScript)
//
// Step 1: Initialize CSRF token when app loads
// ```javascript
// async function initCSRF() {
//   const response = await fetch('/api/v1/csrf-token', {
//     credentials: 'include' // Important: send cookies
//   });
//   const data = await response.json();
//   console.log('CSRF token initialized:', data.csrf_token);
// }
// initCSRF();
// ```
//
// Step 2: Utility function to get cookie value
// ```javascript
// function getCookie(name) {
//   const value = `; ${document.cookie}`;
//   const parts = value.split(`; ${name}=`);
//   if (parts.length === 2) return parts.pop().split(';').shift();
//   return null;
// }
// ```
//
// Step 3: Attach CSRF token to all state-changing requests
//
// Option A: Using Axios interceptor (recommended)
// ```javascript
// axios.interceptors.request.use((config) => {
//   if (!['GET', 'HEAD', 'OPTIONS'].includes(config.method.toUpperCase())) {
//     const csrfToken = getCookie('csrf_token');
//     if (csrfToken) {
//       config.headers['X-CSRF-Token'] = csrfToken;
//     }
//   }
//   return config;
// });
// ```
//
// Option B: Using fetch (manual)
// ```javascript
// async function transferMoney(receiver, amount) {
//   const csrfToken = getCookie('csrf_token');
//
//   const response = await fetch('/api/v1/transactions/transfer', {
//     method: 'POST',
//     headers: {
//       'Content-Type': 'application/json',
//       'X-CSRF-Token': csrfToken // Include CSRF token
//     },
//     credentials: 'include', // Send cookies
//     body: JSON.stringify({ receiver, amount })
//   });
//
//   return response.json();
// }
// ```
//
// Option C: Using React Query
// ```typescript
// const transferMutation = useMutation({
//   mutationFn: async (data: { receiver: string; amount: number }) => {
//     const csrfToken = getCookie('csrf_token');
//     const response = await fetch('/api/v1/transactions/transfer', {
//       method: 'POST',
//       headers: {
//         'Content-Type': 'application/json',
//         'X-CSRF-Token': csrfToken,
//       },
//       credentials: 'include',
//       body: JSON.stringify(data),
//     });
//     if (!response.ok) throw new Error('Transfer failed');
//     return response.json();
//   },
// });
// ```

// SetupCSRFProtectionWithAuth shows how to combine CSRF protection with JWT authentication.
// This is the recommended setup for GoWallet microservices.
func SetupCSRFProtectionWithAuthExample() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.ErrorHandler())

	// Public routes (no auth, no CSRF)
	public := r.Group("/api/v1")
	{
		public.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})
	}

	// Protected routes (JWT auth required, CSRF for cookie-based session)
	// Note: If you ONLY use Bearer token (Authorization header), CSRF is NOT needed
	// CSRF is only needed if you store auth in cookies
	protected := r.Group("/api/v1")
	// protected.Use(middleware.AuthMiddleware(rdb)) // Uncomment if using JWT auth
	{
		// GET endpoints - safe methods don't need CSRF
		protected.GET("/profile", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"user": "john@example.com"})
		})

		// State-changing endpoints - these need CSRF if using cookie-based auth
		// If using Bearer token ONLY, you can skip CSRF middleware for these
		csrfProtected := protected.Group("")
		csrfProtected.Use(middleware.CSRFMiddleware())
		{
			csrfProtected.POST("/transactions", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})
			csrfProtected.PUT("/profile", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})
			csrfProtected.DELETE("/account", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})
		}
	}

	return r
}

// Important Notes:
//
// 1. When to use CSRF protection:
//    - ✅ Web app with cookie-based sessions
//    - ✅ JWT stored in HttpOnly cookie
//    - ✅ Any endpoint that uses cookies for authentication
//    - ❌ Mobile app with Bearer token in Authorization header only
//    - ❌ Microservice-to-microservice API calls (use API keys instead)
//
// 2. SameSite cookie configuration:
//    - SameSite=Strict: Maximum security, never sent cross-site (use for banking apps)
//    - SameSite=Lax: Good balance, sent on top-level navigation only (recommended for most apps)
//    - SameSite=None: Required for third-party cookies (use with Secure=true)
//
// 3. HTTPS is required:
//    - Set Secure=true on all auth cookies in production
//    - For local development over HTTP, set Secure=false
//
// 4. CSRF token rotation:
//    - Token is valid for 24 hours (configured in SetCSRFCookie)
//    - Frontend should handle 403 CSRF errors by refreshing the token
//    - Consider rotating token on login/logout
//
// 5. Testing with Postman/cURL:
//    - GET /api/v1/csrf-token to obtain token
//    - Copy csrf_token from Set-Cookie header
//    - Include as both Cookie and X-CSRF-Token header in POST requests

// add this in api-gateway if want to implement csrf
func apiGatewayMain() {
	r := gin.New()
	//    CSRF middleware implements Double Submit Cookie protection
	r.Use(middleware.CSRFMiddleware())

	// CSRF token retrieval endpoint
	r.GET("/api/v1/csrf-token", func(c *gin.Context) {
		token, err := security.GetCSRFCookie(c)
		if err != nil || token == "" {
			token, err = security.RotateCSRFToken(c)
			if err != nil {
				c.Error(err)
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"csrf_token": token,
			"message":    "Include this token as X-CSRF-Token header in all state-changing requests",
		})
	})
}
