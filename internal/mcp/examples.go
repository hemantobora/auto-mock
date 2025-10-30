package mcp

type ExamplePrompt struct {
	Name        string
	Description string
}

var Examples = []ExamplePrompt{
	{
		Name: "REST - User Management API",
		Description: `User management API for an authentication service.

Endpoints:
- POST /auth/signup — creates a new user with email, name, password.
- POST /auth/login — returns JWT token and refresh token.
- POST /auth/refresh — issues a new access token using refresh token.
- GET /me — returns the logged-in user profile. Requires Authorization: Bearer token.
- PUT /me — updates name and password.
- GET /users?page&limit — returns paginated list of users (admin only).
- DELETE /users/{id} — deletes a user (admin only).

All successful responses use Content-Type: application/json.
Error responses follow: { "error": { "code": "<CODE>", "message": "<DETAIL>" } }.
Include a few delays (e.g., 50–200ms) and one 429 Too Many Requests case for /auth/login.
Use realistic timestamps (ISO 8601) and deterministic IDs like u_1001.`,
	},
	{
		Name: "REST - E-Commerce Catalog API",
		Description: `Catalog API for an online store.

Endpoints:
- GET /products — list products with pagination (page, limit).
- GET /products/{id} — fetch single product by id.
- POST /products — create new product (admin only).
- PUT /products/{id} — update product price or stock.
- DELETE /products/{id} — delete product (admin only).
- GET /categories — list categories.

Each product: id, name, description, price, stock, categoryId.
Include one 404 case for /products/{id}.
Include a Velocity template in /products to echo request header "User-Agent".`,
	},
	{
		Name: "GraphQL - Blog Platform API",
		Description: `GraphQL API for a blogging platform.

Single endpoint: POST /graphql

Queries:
- getPosts(limit:Int,page:Int): returns posts { id, title, author { id name }, createdAt }.
- getPost(id:ID!): returns one post with comments.
Mutations:
- createPost(input:{title,content,authorId}): returns new post.
- deletePost(id:ID!): deletes a post, returns success:true.

Auth required for create/delete (Bearer token).
Include one error case for invalid id.
Include Velocity example that echoes request variable $!variables.limit.`,
	},
	{
		Name: "GraphQL - Inventory API",
		Description: `GraphQL inventory API.

Single endpoint: POST /graphql

Queries:
- products(limit:Int): returns { id, name, price, stock }.
- productById(id:ID!): returns a product or null.
Mutations:
- updateStock(id:ID!, stock:Int!): returns updated product.
- createProduct(input:{name,price,stock}): returns new product.

All requests JSON-only, matchType: ONLY_MATCHING_FIELDS.
Include valid and invalid productId cases and realistic 200/404 responses.`,
	},
}
