package main

import (
	"context"
	"log"

	"github.com/theWebPartyTime/server/internal/config"
	"github.com/theWebPartyTime/server/internal/handlers"
	"github.com/theWebPartyTime/server/internal/repository"
	"github.com/theWebPartyTime/server/internal/repository/postgres"
	"github.com/theWebPartyTime/server/internal/service"

	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	GinAuthMiddleware "github.com/theWebPartyTime/server/internal/auth"
	migrations "github.com/theWebPartyTime/server/internal/db"
	localStorage "github.com/theWebPartyTime/server/internal/storage/local"
)

func main() {
	router := gin.Default()
	router.SetTrustedProxies(nil)

	node, err := centrifuge.New(centrifugeMainConfig())

	if err != nil {
		log.Fatal(err)
	}

	node.OnConnect(func(client *centrifuge.Client) {
		client.OnPresenceStats(onPresenceStats())
		client.OnRPC(onRPC(node, client))
		client.OnSubscribe(onSubscribe(node, client))
		client.OnUnsubscribe(onUnsubscribe(node, client))
		client.OnMessage(onMessage(client))
	})

	if err := node.Run(); err != nil {
		log.Fatal(err)
	}

	wsHandler := centrifuge.NewWebsocketHandler(node, wsMainConfig())

	router.GET("/", root)
	router.GET(socketPath, gin.WrapH(auth(wsHandler)))

	ctx := context.Background()
	config := config.LoadConfig()

	err = repository.InitDB(ctx, config)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	db := repository.GetDB()

	if err := migrations.RunMigrations(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}
	defer repository.CloseDB()

	deps := NewDependencies(db, config)
	authHandler := deps.NewAuthHandler()
	scriptsHandler := deps.NewScriptsHandler()
	authMiddleware := deps.NewAuthMiddleware()

	router.Use(corsMiddleware())
	router.OPTIONS("/*path", func(c *gin.Context) {
		c.AbortWithStatus(204)
	})

	authGroup := router.Group("/auth")

	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/refresh", authHandler.RefreshToken)

	scriptsGroup := router.Group("/scripts", authMiddleware.GinAuthMiddleware())

	scriptsGroup.GET("/user", scriptsHandler.UserScripts)
	scriptsGroup.GET("/public", scriptsHandler.PublicScripts)
	scriptsGroup.POST("/", scriptsHandler.UploadScript)
	scriptsGroup.PUT("/:script_hash", scriptsHandler.UpdateScript)

	router.Run("0.0.0.0:8080")
}

type Dependencies struct {
	db     *gorm.DB
	config config.Config
}

func NewDependencies(db *gorm.DB, config config.Config) *Dependencies {
	return &Dependencies{
		db:     db,
		config: config,
	}
}

func (d *Dependencies) NewAuthHandler() *handlers.AuthHandler {
	userRepo := postgres.NewPostgresUserRepository(d.db)
	authService := service.NewAuthService(userRepo)
	return handlers.NewAuthHandler(authService, []byte(d.config.JWTSecret))

}

func (d *Dependencies) NewScriptsHandler() *handlers.ScriptsHandler {
	scriptsRepo := postgres.NewPostgresScriptsRepository(d.db)
	scriptsStorage := localStorage.NewLocalFilesStorage("/uploads/scripts/", ".toml")
	imagesStorage := localStorage.NewLocalFilesStorage("/uploads/images/", ".jpg")
	scriptsService := service.NewScriptsService(scriptsRepo, scriptsStorage, imagesStorage)
	return handlers.NewScriptsHandler(scriptsService)
}

func (d *Dependencies) NewAuthMiddleware() *GinAuthMiddleware.JWTMiddleware {
	return GinAuthMiddleware.NewJWTMiddleware([]byte(d.config.JWTSecret))
}

func corsMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5174",
			"http://127.0.0.1:5174",
			"http://webparty.fun",
			"https://webparty.fun",
		},
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
