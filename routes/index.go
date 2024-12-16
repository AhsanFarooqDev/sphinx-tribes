package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/rs/cors"
	"github.com/urfave/negroni"

	"github.com/stakwork/sphinx-tribes/auth"
	"github.com/stakwork/sphinx-tribes/db"
	"github.com/stakwork/sphinx-tribes/handlers"
)

// NewRouter creates a chi router
func NewRouter() *http.Server {
	r := initChi()
	tribeHandlers := handlers.NewTribeHandler(db.DB)
	authHandler := handlers.NewAuthHandler(db.DB)
	channelHandler := handlers.NewChannelHandler(db.DB)
	botHandler := handlers.NewBotHandler(db.DB)
	bHandler := handlers.NewBountyHandler(http.DefaultClient, db.DB)

	r.Mount("/tribes", TribeRoutes())
	r.Mount("/bots", BotsRoutes())
	r.Mount("/bot", BotRoutes())
	r.Mount("/people", PeopleRoutes())
	r.Mount("/person", PersonRoutes())
	r.Mount("/connectioncodes", ConnectionCodesRoutes())
	r.Mount("/github_issue", GithubIssuesRoutes())
	r.Mount("/gobounties", BountyRoutes())
	r.Mount("/workspaces", WorkspaceRoutes())
	r.Mount("/metrics", MetricsRoutes())
	r.Mount("/features", FeatureRoutes())
	r.Mount("/workflows", WorkflowRoutes())
	r.Mount("/bounties/ticket", TicketRoutes())
	r.Mount("/hivechat", ChatRoutes())

	r.Group(func(r chi.Router) {
		r.Get("/tribe_by_feed", tribeHandlers.GetFirstTribeByFeed)
		r.Get("/leaderboard/{tribe_uuid}", handlers.GetLeaderBoard)
		r.Get("/tribe_by_un/{un}", tribeHandlers.GetTribeByUniqueName)
		r.Get("/tribes_by_owner/{pubkey}", tribeHandlers.GetTribesByOwner)

		r.Get("/search/bots/{query}", botHandler.SearchBots)
		r.Get("/podcast", handlers.GetPodcast)
		r.Get("/feed", handlers.GetGenericFeed)
		r.Post("/feed/download", handlers.DownloadYoutubeFeed)
		r.Get("/search_podcasts", handlers.SearchPodcasts)
		r.Get("/search_podcast_episodes", handlers.SearchPodcastEpisodes)
		r.Get("/search_youtube", handlers.SearchYoutube)
		r.Get("/search_youtube_videos", handlers.SearchYoutubeVideos)
		r.Get("/youtube_videos", handlers.YoutubeVideosForChannel)
		r.Get("/admin_pubkeys", handlers.GetAdminPubkeys)

		r.Get("/ask", db.Ask)
		r.Get("/poll/{challenge}", db.Poll)
		r.Post("/save", db.PostSave)
		r.Get("/save/{key}", db.PollSave)
		r.Get("/migrate_bounties", handlers.MigrateBounties)
		r.Get("/websocket", handlers.HandleWebSocket)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.PubKeyContext)
		r.Post("/channel", channelHandler.CreateChannel)
		r.Post("/leaderboard/{tribe_uuid}", handlers.CreateLeaderBoard)
		r.Put("/leaderboard/{tribe_uuid}", handlers.UpdateLeaderBoard)
		r.Put("/tribe", tribeHandlers.CreateOrEditTribe)
		r.Put("/tribestats", handlers.PutTribeStats)
		r.Delete("/tribe/{uuid}", tribeHandlers.DeleteTribe)
		r.Put("/tribeactivity/{uuid}", handlers.PutTribeActivity)
		r.Put("/tribepreview/{uuid}", tribeHandlers.SetTribePreview)
		r.Post("/verify/{challenge}", db.Verify)
		r.Post("/badges", handlers.AddOrRemoveBadge)
		r.Delete("/channel/{id}", channelHandler.DeleteChannel)
		r.Delete("/ticket/{pubKey}/{created}", handlers.DeleteTicketByAdmin)
		r.Get("/poll/invoice/{paymentRequest}", bHandler.PollInvoice)
		r.Post("/meme_upload", handlers.MemeImageUpload)
		r.Get("/admin/auth", authHandler.GetIsAdmin)
	})

	r.Group(func(r chi.Router) {
		r.Get("/lnauth_login", handlers.ReceiveLnAuthData)
		r.Get("/lnauth", handlers.GetLnurlAuth)
		r.Get("/refresh_jwt", authHandler.RefreshToken)
		r.Post("/invoices", handlers.GenerateInvoice)
		r.Post("/budgetinvoices", tribeHandlers.GenerateBudgetInvoice)
	})

	PORT := os.Getenv("PORT")
	if PORT == "" {
		PORT = "5002"
	}

	server := &http.Server{Addr: ":" + PORT, Handler: r}

	go func() {
		fmt.Println("Listening on port " + PORT)
		if err := server.ListenAndServe(); err != nil {
			fmt.Println("server err:", err.Error())
		}
	}()

	return server
}

type extractResponse struct {
	Pubkey string `json:"pubkey"`
	Valid  bool   `json:"valid"`
}

func getFromAuth(path string) (*extractResponse, error) {

	authURL := "http://auth:9090"
	resp, err := http.Get(authURL + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body2, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var inter map[string]interface{}
	err = json.Unmarshal(body2, &inter)
	if err != nil {
		return nil, err
	}
	pubkey, _ := inter["pubkey"].(string)
	valid, _ := inter["valid"].(bool)
	return &extractResponse{
		Pubkey: pubkey,
		Valid:  valid,
	}, nil
}

// Middleware to handle InternalServerError
func internalServerErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr := negroni.NewResponseWriter(w)
		next.ServeHTTP(rr, r)

		if rr.Status() == http.StatusInternalServerError {
			fmt.Printf("Internal Server Error: %s %s\n", r.Method, r.URL.Path)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
}


func initChi() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(internalServerErrorHandler)
	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-User", "authorization", "x-jwt", "Referer", "User-Agent"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(cors.Handler)
	r.Use(middleware.Timeout(60 * time.Second))
	return r
}
