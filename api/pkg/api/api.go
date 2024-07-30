package api

import (
	"encoding/json"
	"fmt"
	"log"
  "strings"
	"net/http"
	"os"
  
	"github.com/google/uuid"
  "github.com/rs/cors"
	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
  storage "api/pkg/storage"
  types "api/pkg/types"
)

type APIServer struct {
	listenAddr string
	store      storage.Storage
}

func NewAPIServer(listenAddr string, store storage.Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()

	router.HandleFunc("/login", makeHTTPHandleFunc(s.handleLogin))
	router.HandleFunc("/sign-up", withJWTAuth(makeHTTPHandleFunc(s.handleSignUp), s.store))
	router.HandleFunc("/delete-account/{username}", withJWTAuth(makeHTTPHandleFunc(s.handleDeleteAccount), s.store))
	
  router.HandleFunc("/software/id/{software-id}", makeHTTPHandleFunc(s.handleGetSoftwareByID))
  router.HandleFunc("/software/{software-id}", withJWTAuth(makeHTTPHandleFunc(s.handleSoftware), s.store))
  router.HandleFunc("/software", makeHTTPHandleFunc(s.handleGetSoftware))
  
  router.HandleFunc("/software-likes/{software-id}/user/{user-id}", withJWTAuth(makeHTTPHandleFunc(s.handleSoftwareLike), s.store))
  router.HandleFunc("/software-likes/{software-id}", makeHTTPHandleFunc(s.handleGetSoftwareLikesByID))

	log.Println("JSON API server running on port: ", s.listenAddr)

  c := cors.New(cors.Options{
    AllowedOrigins: []string{"http://localhost:4321"},
    AllowedMethods: []string{
      http.MethodGet,
      http.MethodPost,
      http.MethodPut,
      http.MethodPatch,
      http.MethodDelete,
      http.MethodOptions,
      http.MethodHead,
    },
    AllowedHeaders: []string{"*"},
    AllowCredentials: true,
  })

  handler := c.Handler(router)

	http.ListenAndServe(s.listenAddr, handler)
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(v)
}

func createJWT(user *types.User) (string, error) {
	claims := &jwt.MapClaims{
		"expiresAt":     15000,
		"username":      user.Username,
	}

	secret := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

func permissionDenied(w http.ResponseWriter) {
	WriteJSON(w, http.StatusForbidden, ApiError{Error: "permission denied"})
}

func withJWTAuth(handlerFunc http.HandlerFunc, s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("calling JWT auth middleware")

		tokenString := strings.Split(r.Header.Get("Authorization"), "Bearer ")[1]
		token, err := validateJWT(tokenString)
		if err != nil {
			permissionDenied(w)
			return
		}
		if !token.Valid {
			permissionDenied(w)
			return
		}

		claims := token.Claims.(jwt.MapClaims)

    type Request struct {
      UserID       uuid.UUID   `json:"userId"`
      Username     string      `json:"username"`
    }
    req := new(Request)
    if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			permissionDenied(w)
      return
    }

		if req.Username != claims["username"] && req.UserID != claims["userID"] {
			permissionDenied(w)
			return
		}

		if err != nil {
			WriteJSON(w, http.StatusForbidden, ApiError{Error: "invalid token"})
			return
		}

		handlerFunc(w, r)
	}
}

func validateJWT(tokenString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")

	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return []byte(secret), nil
	})
}

type apiFunc func(http.ResponseWriter, *http.Request) error

type ApiError struct {
	Error string `json:"error"`
}

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, ApiError{Error: err.Error()})
		}
	}
}

func getID(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}