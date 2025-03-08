// AuthMiddleware is a middleware function that checks if the request has a valid JWT token
func (m *Middleware) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from cookie
		tokenString, err := c.Cookie("jwt_token")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(m.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.SetCookie("jwt_token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.SetCookie("jwt_token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("userID", uint(claims["user_id"].(float64)))
		c.Set("email", claims["email"].(string))
		c.Set("username", claims["username"].(string))
		c.Set("isAdmin", claims["is_admin"].(bool))

		c.Next()
	}
} 