package routers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andrewbelo/bootdotdev-chirpy/internal/db"
	"github.com/golang-jwt/jwt/v5"
)

type userRequest struct {
	Password         string `json:"password"`
	Email            string `json:"email"`
	ExpiresInSeconds int    `json:"expires_in_seconds,omitempty"`
}
type tokenRequest struct {
	AccessToken string `json:"token"`
}
type userResponse struct {
	Email       string `json:"email"`
	ID          int    `json:"id"`
	IsChirpyRed bool   `json:"is_chirpy_red"`
}
type userResponseWithToken struct {
	Email        string `json:"email"`
	ID           int    `json:"id"`
	Token        string `json:"token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IsChirpyRed  bool   `json:"is_chirpy_red"`
}

func (cfg *ApiConfig) newJWTToken(id int, token_type string) (string, error) {
	if token_type != "access" && token_type != "refresh" {
		return "", errors.New("invalid token type")
	}

	expires_in := time.Hour * time.Duration(24*60)
	issuer := "chirpy-refresh"
	if token_type == "access" {
		expires_in = time.Hour
		issuer = "chirpy-access"
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   strconv.Itoa(id),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expires_in)),
	})
	return token.SignedString([]byte(cfg.JWTSecret))
}

func (cfg *ApiConfig) checkAccessJWTInHeader(auth_header string) (int, error) {
	return cfg.checkJWT(strings.TrimPrefix(auth_header, "Bearer "), "access")
}

func (cfg *ApiConfig) checkRefreshJWTInHeader(auth_header string) (int, error) {
	return cfg.checkJWT(strings.TrimPrefix(auth_header, "Bearer "), "access")
}

func (cfg *ApiConfig) checkJWT(tokenString string, token_type string) (int, error) {
	if token_type != "access" && token_type != "refresh" {
		return 0, errors.New("invalid token type")
	}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

	if err != nil {
		return 0, err
	}
	claims := token.Claims.(*jwt.RegisteredClaims)
	issuer := "chirpy-refresh"
	if token_type == "access" {
		issuer = "chirpy-access"
	}
	if claims.Issuer != issuer {
		return 0, errors.New("invalid issuer")
	}
	if claims.ExpiresAt.Time.Before(time.Now()) {
		return 0, errors.New("token expired")
	}
	userID, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (cfg *ApiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	userR := userRequest{ExpiresInSeconds: 24 * 3600}
	err := decoder.Decode(&userR)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	user, err := cfg.DB.LogInUser(userR.Email, userR.Password)
	if errors.Is(err, db.ErrInvalidPassword) || errors.Is(err, db.ErrUserNotFound) {
		marshalError(w, err, http.StatusUnauthorized)
		return
	}
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	tokenString, err := cfg.newJWTToken(user.ID, "access")
	refreshTokenString, err := cfg.newJWTToken(user.ID, "refresh")
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	marshallOK(w, userResponseWithToken{
		user.Email, user.ID, tokenString, refreshTokenString, user.IsChirpyRed,
	})
}

func (cfg *ApiConfig) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	userR := userRequest{}
	err := decoder.Decode(&userR)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}

	tokenString := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	userID, err := cfg.checkJWT(tokenString, "access")
	if err != nil {
		marshalError(w, err, http.StatusUnauthorized)
		return
	}

	log.Println("Updating user")
	user, err := cfg.DB.UpdateUser(userID, userR.Email, userR.Password)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	log.Println("User updated")
	marshallOK(w, userResponse{user.Email, user.ID, user.IsChirpyRed})
}

func (cfg *ApiConfig) polkaWebhook(w http.ResponseWriter, r *http.Request) {
	var polkaRequest struct {
		Event string `json:"event"`
		Data  struct {
			UserID int `json:"user_id"`
		} `json:"data"`
	}
	const UserUpgrade = "user.upgraded"

	apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "ApiKey ")
	if apiKey != cfg.PolkaApiKey {
		marshalError(w, errors.New("Invalid Polka credentials"), http.StatusUnauthorized)
		return
	}

	err := json.NewDecoder(r.Body).Decode(&polkaRequest)
	if err != nil {
		marshalError(w, err, http.StatusBadRequest)
		return
	}
	if polkaRequest.Event != UserUpgrade {
		marshallEmptyOK(w)
		return
	}

	log.Printf("request: %v", polkaRequest)
	log.Printf("user_id: %d", polkaRequest.Data.UserID)
	err = cfg.DB.UpgradeUser(polkaRequest.Data.UserID)
	if errors.Is(err, db.ErrUserNotFound) {
		marshalError(w, err, http.StatusNotFound)
		return
	}
	if err != nil {
		log.Fatal(err)
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	marshallEmptyOK(w)
}

func (cfg *ApiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	userR := userRequest{}
	err := decoder.Decode(&userR)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}

	log.Println("Creating user")
	user, err := cfg.DB.CreateUser(userR.Email, userR.Password)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	log.Println("User created")
	marshallCreated(w, userResponse{user.Email, user.ID, user.IsChirpyRed})
}

func (cfg *ApiConfig) revokeTokenHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	_, err := cfg.checkJWT(tokenString, "refresh")
	if err != nil {
		marshalError(w, err, http.StatusUnauthorized)
		return
	}
	err = cfg.DB.RevokeToken(tokenString)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	log.Println("Token revoked")
	marshallEmptyOK(w)
}

func (cfg *ApiConfig) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	userID, err := cfg.checkJWT(tokenString, "refresh")
	if err != nil {
		marshalError(w, err, http.StatusUnauthorized)
		return
	}

	err = cfg.DB.CheckToken(tokenString)
	if err != nil {
		marshalError(w, err, http.StatusUnauthorized)
		return
	}

	log.Println("Refreshing token")
	tokenString, err = cfg.newJWTToken(userID, "access")
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	log.Println("Token refreshed")
	marshallOK(w, userResponseWithToken{Token: tokenString})
}
