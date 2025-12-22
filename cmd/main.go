package main

import (
	"context"
	"log"
	"server/internal/config"
	"server/internal/handlers"
	"server/internal/repository"
	"server/internal/repository/postgres"
	"server/internal/service"

	"github.com/centrifugal/centrifuge"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	GinAuthMiddleware "server/internal/auth"
	migrations "server/internal/db"
	localStorage "server/internal/storage/local"
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

	router.Use(authMiddleware.GinAuthMiddleware()).GET("/scripts/user", scriptsHandler.UserScripts)
	router.Use(authMiddleware.GinAuthMiddleware()).GET("/scripts/public", scriptsHandler.PublicScripts)
	router.Use(authMiddleware.GinAuthMiddleware()).POST("/scripts", scriptsHandler.UploadScript)
	router.Use(authMiddleware.GinAuthMiddleware()).PUT("/scripts/:script_hash", scriptsHandler.UpdateScript)

	router.POST("/auth/login", authHandler.Login)
	router.POST("/auth/register", authHandler.Register)
	router.POST("/auth/refresh", authHandler.RefreshToken)

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
