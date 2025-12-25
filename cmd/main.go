package main

import (
	"context"
	"log"
	"os"

	"github.com/theWebPartyTime/server/internal/config"
	"github.com/theWebPartyTime/server/internal/handlers"
	"github.com/theWebPartyTime/server/internal/repository"
	"github.com/theWebPartyTime/server/internal/repository/postgres"
	"github.com/theWebPartyTime/server/internal/service"

	"github.com/centrifugal/centrifuge"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	GinAuthMiddleware "github.com/theWebPartyTime/server/internal/auth"
	migrations "github.com/theWebPartyTime/server/internal/db"
	localStorage "github.com/theWebPartyTime/server/internal/storage/local"
)

func main() {
	dir, _ := os.Getwd()
	log.Printf(dir)

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
		client.OnDisconnect(onDisconnect(client))
		client.OnMessage(onMessage(node, client))
	})

	if err := node.Run(); err != nil {
		log.Fatal(err)
	}

	wsHandler := centrifuge.NewWebsocketHandler(node, wsMainConfig())

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

	router.GET("/", root)
	router.GET(socketPath,
		gin.WrapH(GinAuthMiddleware.CentrifugeAuthMiddleware(wsHandler)))

	group := router.Group("/scripts/", authMiddleware.GinAuthMiddleware())
	group.GET("/user", scriptsHandler.UserScripts)
	group.GET("/public", scriptsHandler.PublicScripts)
	group.POST("/", scriptsHandler.UploadScript)
	group.PUT("/:script_hash", scriptsHandler.UpdateScript)

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
	scriptsStorage := localStorage.NewLocalFilesStorage("/uploads/scripts/", ".webparty")
	imagesStorage := localStorage.NewLocalFilesStorage("/uploads/images/", ".jpg")
	scriptsService := service.NewScriptsService(scriptsRepo, scriptsStorage, imagesStorage)
	return handlers.NewScriptsHandler(scriptsService)
}

func (d *Dependencies) NewAuthMiddleware() *GinAuthMiddleware.JWTMiddleware {
	return GinAuthMiddleware.NewJWTMiddleware([]byte(d.config.JWTSecret))
}
