// Package docs contains the aggregated OpenAPI specification for all
// microservice endpoints proxied through the API Gateway.
package docs

// @title           GoWallet API (Microservices)
// @version         2.0
// @description     Unified API documentation for all GoWallet microservices proxied through the API Gateway.
// @termsOfService  http://swagger.io/terms/

// @contact.name   GoWallet API Support
// @contact.email  support@gowallet.com

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// ============================================
// Auth Service Endpoints
// ============================================

// @Summary      Login
// @Description  Authenticate user with email and password. Returns JWT access + refresh tokens.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body object{email=string,password=string} true "Login credentials"
// @Success      200  {object}  object{success=bool,data=object{access_token=string,refresh_token=string}}
// @Failure      401  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /auth/login [post]

// @Summary      Refresh Token
// @Description  Exchange a valid refresh token for a new access + refresh token pair.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body object{refresh_token=string} true "Refresh token"
// @Success      200  {object}  object{success=bool,data=object{access_token=string,refresh_token=string}}
// @Failure      401  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /auth/refresh [post]

// @Summary      Logout
// @Description  Blacklist the current access token so it can no longer be used.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{success=bool,message=string}
// @Failure      401  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /auth/logout [post]

// ============================================
// User Service Endpoints
// ============================================

// @Summary      Register New User
// @Description  Create a new user account with a default wallet.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body object{full_name=string,email=string,password=string} true "Registration payload"
// @Success      201  {object}  object{success=bool,data=object{id=string,full_name=string,email=string,role=string}}
// @Failure      400  {object}  object{success=bool,error=object{code=string,message=string}}
// @Failure      409  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /users/register [post]

// @Summary      Get User Profile
// @Description  Get the authenticated user's profile.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID (UUID)"
// @Success      200  {object}  object{success=bool,data=object{id=string,full_name=string,email=string,role=string}}
// @Failure      404  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /users/{id} [get]

// ============================================
// Wallet Service Endpoints
// ============================================

// @Summary      Get Wallet Balance
// @Description  Get the authenticated user's wallet balance.
// @Tags         wallets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{success=bool,data=object{id=string,user_id=string,balance=float64,version=int}}
// @Failure      404  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /wallets/me [get]

// ============================================
// Transaction Service Endpoints
// ============================================

// @Summary      Transfer Funds
// @Description  Transfer money from the authenticated user's wallet to another user.
// @Tags         transactions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body object{receiver_email=string,amount=float64,description=string,idempotency_key=string} true "Transfer payload"
// @Success      200  {object}  object{success=bool,data=object{id=string,status=string,amount=float64}}
// @Failure      400  {object}  object{success=bool,error=object{code=string,message=string}}
// @Failure      409  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /transactions/transfer [post]

// @Summary      Get Transaction History
// @Description  List all transactions for the authenticated user.
// @Tags         transactions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200  {object}  object{success=bool,data=[]object{id=string,sender_wallet_id=string,receiver_wallet_id=string,amount=float64,status=string}}
// @Router       /transactions [get]

// ============================================
// Payment Service Endpoints
// ============================================

// @Summary      Create Payment
// @Description  Initiate a payment (top-up or withdrawal).
// @Tags         payments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body object{amount=float64,payment_method=string,description=string} true "Payment payload"
// @Success      201  {object}  object{success=bool,data=object{id=string,status=string,amount=float64}}
// @Failure      400  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /payments [post]

// @Summary      Get Payment Status
// @Description  Check the status of a payment by ID.
// @Tags         payments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Payment ID (UUID)"
// @Success      200  {object}  object{success=bool,data=object{id=string,status=string,amount=float64}}
// @Failure      404  {object}  object{success=bool,error=object{code=string,message=string}}
// @Router       /payments/{id} [get]
