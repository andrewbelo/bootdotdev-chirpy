package db

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidPassword = errors.New("Invalid password")
var ErrUserNotFound = errors.New("User not found")
var ErrUnauthorized = errors.New("You do not have permission to perform this action")

type DB struct {
	path string
	mx   sync.RWMutex
}

type DBSchema struct {
	Chirps        map[int]Chirp        `json:"chirps"`
	Users         map[int]User         `json:"users"`
	RevokedTokens map[string]time.Time `json:"revoked_tokens"`
}

type Chirp struct {
	AuthorID int    `json:"author_id"`
	Body     string `json:"body"`
	ID       int    `json:"id"`
	Deleted  bool
}

type User struct {
	Email       string `json:"email"`
	ID          int    `json:"id"`
	Password    string `json:"password"`
	IsChirpyRed bool   `json:"is_chirpy_red"`
}

func NewDB(path string, debug bool) (*DB, error) {
	db := &DB{
		path: path,
	}
	err := db.ensureDB(debug)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) loadDB() (DBSchema, error) {
	db.mx.RLock()
	defer db.mx.RUnlock()

	f, err := os.ReadFile(db.path)
	if err != nil {
		return DBSchema{}, err
	}

	dbSchema := DBSchema{}
	err = json.Unmarshal(f, &dbSchema)
	if err != nil {
		return DBSchema{}, err
	}

	return dbSchema, nil
}

func (db *DB) writeDB(dbSchema DBSchema) error {
	db.mx.Lock()
	defer db.mx.Unlock()
	f, err := json.Marshal(dbSchema)
	if err != nil {
		return err
	}

	err = os.WriteFile(db.path, f, 0666)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) CreateChirp(body string, user_id int) (Chirp, error) {
	dbSchema, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	id := len(dbSchema.Chirps) + 1
	chirp := Chirp{
		ID: id, Body: body, AuthorID: user_id, Deleted: false,
	}
	dbSchema.Chirps[id] = chirp
	err = db.writeDB(dbSchema)
	if err != nil {
		return Chirp{}, err
	}
	return chirp, nil
}

func (db *DB) printDB() error {
	dbSchema, err := db.loadDB()
	if err != nil {
		return err
	}
	log.Println("Chirps")
	for id, chirp := range dbSchema.Chirps {
		log.Printf("Chirp %d: %v", id, chirp)
	}
	return nil
}

func (db *DB) UpgradeUser(user_id int) error {
	log.Printf("Updating user %d", user_id)
	dbSchema, err := db.loadDB()
	if err != nil {
		return err
	}
	user := dbSchema.Users[user_id]
	user.IsChirpyRed = true
	dbSchema.Users[user_id] = user
	err = db.writeDB(dbSchema)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) DeleteChirp(chirp_id int, user_id int) error {
	dbSchema, err := db.loadDB()
	if err != nil {
		return err
	}
	chirp := dbSchema.Chirps[chirp_id]
	if chirp.AuthorID != user_id {
		return ErrUnauthorized
	}
	chirp.Deleted = true
	dbSchema.Chirps[chirp_id] = chirp
	err = db.writeDB(dbSchema)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) CreateUser(email string, password string) (User, error) {
	dbSchema, err := db.loadDB()
	if err != nil {
		return User{}, err
	}

	println("Creating user")
	id := len(dbSchema.Users) + 1
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password), bcrypt.DefaultCost,
	)
	if err != nil {
		return User{}, err
	}
	user := User{
		ID: id, Email: email, Password: string(passwordHash), IsChirpyRed: false,
	}
	dbSchema.Users[id] = user

	err = db.writeDB(dbSchema)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (db *DB) UpdateUser(id int, email string, password string) (User, error) {
	dbSchema, err := db.loadDB()
	if err != nil {
		return User{}, err
	}

	println("Updating user")
	user, ok := dbSchema.Users[id]
	if !ok {
		return User{}, errors.New("User not found")
	}
	user.Email = email
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password), bcrypt.DefaultCost,
	)
	if err != nil {
		return User{}, err
	}
	user.Password = string(passwordHash)
	dbSchema.Users[id] = user

	err = db.writeDB(dbSchema)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (db *DB) LogInUser(email string, password string) (User, error) {
	dbSchema, err := db.loadDB()
	if err != nil {
		return User{}, err
	}

	println("Logging in user")
	for _, user := range dbSchema.Users {
		if user.Email == email {
			err = bcrypt.CompareHashAndPassword(
				[]byte(user.Password),
				[]byte(password),
			)
			if err != nil {
				return User{}, ErrInvalidPassword
			}
			return user, nil
		}
	}

	return User{}, ErrUserNotFound
}

func (db *DB) GetUser(id int) (User, error) {
	dbSchema, err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	user, ok := dbSchema.Users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (db *DB) GetUsers() ([]User, error) {
	var users []User
	dbSchema, err := db.loadDB()
	if err != nil {
		return users, err
	}
	for i := 1; i <= len(dbSchema.Users); i++ {
		users = append(users, dbSchema.Users[i])
	}

	return users, nil
}

func (db *DB) GetChirpsFilteredAndSorted(
	filter func(Chirp) bool, cmp func(Chirp, Chirp) int,
) ([]Chirp, error) {
	var chirps []Chirp
	dbSchema, err := db.loadDB()
	if err != nil {
		return chirps, err
	}
	for _, chirp := range dbSchema.Chirps {
		log.Printf("chip %v", chirp)
		if filter(chirp) {
			chirps = append(chirps, chirp)
		}
	}
	slices.SortFunc(chirps, cmp)
	return chirps, nil
}

func ascIDOrder(lhs Chirp, rhs Chirp) int {
	return lhs.ID - rhs.ID
}
func descIDOrder(lhs Chirp, rhs Chirp) int {
	return -ascIDOrder(lhs, rhs)
}
func determineOrderFunc(sort_by string) func(Chirp, Chirp) int {
	if sort_by == "desc" {
		return descIDOrder
	}
	return ascIDOrder
}

func (db *DB) GetChirpsFiltered(filter func(Chirp) bool) ([]Chirp, error) {
	return db.GetChirpsFilteredAndSorted(filter, func(lhs, rhs Chirp) int {
		return lhs.ID - rhs.ID
	})
}

func (db *DB) GetChirpsOrderedBy(sort_by string) ([]Chirp, error) {
	return db.GetChirpsFilteredAndSorted(
		func(c Chirp) bool { return !c.Deleted },
		determineOrderFunc(sort_by),
	)
}

func (db *DB) GetChirpsByUser(user_id int, sort_by string) ([]Chirp, error) {
	return db.GetChirpsFilteredAndSorted(
		func(c Chirp) bool {
			return !c.Deleted && c.AuthorID == user_id
		},
		determineOrderFunc(sort_by),
	)
}

func (db *DB) GetChirps() ([]Chirp, error) {
	return db.GetChirpsFiltered(func(c Chirp) bool { return !c.Deleted })
}

func (db *DB) GetChirp(id int) (Chirp, error) {
	dbSchema, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}

	chirp, ok := dbSchema.Chirps[id]
	log.Printf("chip %v", chirp)
	if !ok || chirp.Deleted {
		return Chirp{}, errors.New("Chirp not found")
	}

	return chirp, nil
}

func (db *DB) ensureDB(debug bool) error {
	_, err := os.Stat(db.path)
	if os.IsNotExist(err) || debug {
		dbSchema := DBSchema{
			Chirps:        map[int]Chirp{},
			Users:         map[int]User{},
			RevokedTokens: map[string]time.Time{},
		}
		f, err := os.Create(db.path)
		if err != nil {
			return err
		}
		towrite, err := json.Marshal(dbSchema)
		defer f.Close()
		_, err = f.Write(towrite)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) RevokeToken(token string) error {
	dbSchema, err := db.loadDB()
	if err != nil {
		return err
	}
	dbSchema.RevokedTokens[token] = time.Now()
	err = db.writeDB(dbSchema)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) CheckToken(token string) error {
	dbSchema, err := db.loadDB()
	if err != nil {
		return err
	}
	if _, ok := dbSchema.RevokedTokens[token]; ok {
		return errors.New("Token revoked")
	}
	return nil
}
