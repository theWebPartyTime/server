package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	GinAuthMiddleware "server/internal/auth"
	"server/internal/config"
	migrations "server/internal/db"
	"server/internal/handlers"
	"server/internal/repository"
	"server/internal/repository/postgres"
	"server/internal/service"
	localStorage "server/internal/storage/local"
	"strconv"

	"github.com/centrifugal/centrifuge"
	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var presenters = make(map[string]string)
var gameData = make(map[string]int)

type ChatMessage struct {
	Message   string
	Presenter bool
}

func auth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		context := r.Context()
		credentials := &centrifuge.Credentials{UserID: ""}
		newContext := centrifuge.SetCredentials(context, credentials)
		r = r.WithContext(newContext)
		h.ServeHTTP(w, r)
	})
}

func main() {
	router := gin.Default()
	node, err := centrifuge.New(centrifuge.Config{})

	if err != nil {
		log.Fatal(err)
	}

	node.OnConnect(func(client *centrifuge.Client) {
		client.OnSubscribe(func(e centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
			turquoise := color.RGB(3, 252, 202).SprintFunc()
			log.Printf("| User %v joined channel %v", turquoise(client.ID()), turquoise(e.Channel))
			presenceStatsResult, _ := node.PresenceStats(e.Channel)
			if presenceStatsResult.NumClients == 0 {
				presenters[e.Channel] = client.ID()
				gameData[e.Channel] = rand.IntN(10) + 1
				data, _ := json.Marshal(fmt.Sprintf("Случайное число: %v", gameData[e.Channel]))
				client.Send(data)
			}

			cb(centrifuge.SubscribeReply{
				Options: centrifuge.SubscribeOptions{
					EmitPresence: true,
				},
			}, nil)
		})

		client.OnPresence(func(event centrifuge.PresenceEvent, cb centrifuge.PresenceCallback) {
			cb(centrifuge.PresenceReply{}, nil)
		})

		zero, _ := json.Marshal(0)

		client.OnPublish(func(e centrifuge.PublishEvent, cb centrifuge.PublishCallback) {
			if len(e.Channel) == 1 {
				var chatMessage ChatMessage
				json.Unmarshal(e.Data, &chatMessage)
				isPresenter := client.ID() == presenters[e.Channel]
				chatMessage.Presenter = isPresenter
				if !isPresenter {
					number, err := strconv.ParseInt(chatMessage.Message, 10, 64)
					if err == nil {
						if number != int64(gameData[e.Channel]) {
							msg, _ := json.Marshal("Не угадали")
							client.Send(msg)
							chatMessage.Message = "Число не угадали"
						} else {
							msg, _ := json.Marshal("Угадали")
							client.Send(msg)
							chatMessage.Message = "Число угадали"
						}
					}
				}
				// TODO: custom Marshal method to make struct fields lowercase
				var data, _ = json.Marshal(chatMessage)

				var result, _ = node.Publish(
					e.Channel, data,
				)

				cb(centrifuge.PublishReply{Result: &result}, nil)
				return
			}

			cb(centrifuge.PublishReply{}, nil)
		})

		client.OnDisconnect(func(event centrifuge.DisconnectEvent) {
			node.Publish("online-count", zero)
			node.Publish("online-count", zero)
		})

		node.Publish("online-count", zero)
		node.Publish("online-count", zero)
	})

	if err := node.Run(); err != nil {
		log.Fatal(err)
	}

	wsHandler := centrifuge.NewWebsocketHandler(
		node, centrifuge.WebsocketConfig{
			CheckOrigin: func(r *http.Request) bool {
				// originHeader := r.Header.Get("Origin")
				// if originHeader == "http://localhost:8000" {
				// 	return true }

				return true
			},
		})

	router.GET("/", func(context *gin.Context) {
		context.JSON(200, gin.H{
			"message": "Welcome to WebPartyTime!",
		})
	})

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
	imageHandler := deps.NewImageHandler()
	authMiddleware := deps.NewAuthMiddleware()

	scriptsRouter := router.Group("/scripts")
	scriptsRouter.Use(authMiddleware.GinAuthMiddleware())

	scriptsRouter.GET("/user", scriptsHandler.UserScripts)
	scriptsRouter.GET("/public", scriptsHandler.PublicScripts)
	scriptsRouter.POST("/", scriptsHandler.UploadScript)
	scriptsRouter.PUT("/:script_hash", scriptsHandler.UpdateScript)

	authRouter := router.Group("/auth")
	authRouter.POST("/login", authHandler.Login)
	authRouter.POST("/register", authHandler.Register)
	authRouter.POST("/refresh", authHandler.RefreshToken)

	imageRouter := router.Group("/images")
	imageRouter.Use(authMiddleware.GinAuthMiddleware())

	imageRouter.GET("/:hash", imageHandler.GetMediaByHash)

	router.GET("/w", gin.WrapH(auth(wsHandler)))
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
	scriptsStorage := localStorage.NewLocalFilesStorage("/app/uploads/scripts/", ".toml")
	imagesStorage := localStorage.NewLocalFilesStorage("/app/uploads/images/", ".jpg")
	scriptsService := service.NewScriptsService(scriptsRepo, scriptsStorage, imagesStorage)
	return handlers.NewScriptsHandler(scriptsService)
}

func (d *Dependencies) NewImageHandler() *handlers.AssetsHandler {
	contentType := "image/jpg"
	imageStorage := localStorage.NewLocalFilesStorage("/app/uploads/images/", ".jpg")
	return handlers.NewAssetsHandler(imageStorage, contentType)
}

func (d *Dependencies) NewAuthMiddleware() *GinAuthMiddleware.JWTMiddleware {
	return GinAuthMiddleware.NewJWTMiddleware([]byte(d.config.JWTSecret))
}
