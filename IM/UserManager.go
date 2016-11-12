package IM

import (
	"sync"
	"crypto/md5"
	"time"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

const (
	DefaultExpireTime = (time.Minute * 30)
)

/*
* Secret key is keep by user manager and some other server,
* so that the server can register a user through the secret key
*
* You can also update the secret key dynamically as long as the server
* hold the origin key, and update through function UpdateSecretKey
*/
type UserManager struct {
	mutex sync.RWMutex

	secretKey string
	users map[string]*User

	ticker *time.Ticker
}

func NewUserManager(secretKey string) *UserManager {
	return &UserManager{
		secretKey		: secretKey,
		users			: make(map[string]*User),
	}
}


/*
* A user of IM
*/
type User struct {
	checkCode string
	id string
	createTime time.Time
	expireTime time.Duration

	mutex sync.Mutex
	disabled bool
}
func (u* User) Invalidate() {
	u.mutex.Lock()
	u.disabled = true
	u.mutex.Unlock()
}

func NewCheckCode(id string) (string, error) {
	h := md5.New()
	h.Write([]byte(id + time.Now().String()))
	return hex.EncodeToString(h.Sum(nil)), nil
}

/*
* Register a user with user's id and the secret key
*/
func (m *UserManager) RegisterUser(validation string, id string, expireTime time.Duration) (string, error) {
	m.mutex.RLock()
	sk := m.secretKey
	m.mutex.RUnlock()

	if validation == sk {
		checkCode, err := NewCheckCode(id)
		if err != nil {
			fmt.Println(err)
			return "", err
		}

		m.mutex.Lock()
		m.users[checkCode] = &User{
			id		: id,
			checkCode	: checkCode,
			expireTime	: expireTime,
		}
		m.mutex.Unlock()

		return checkCode, nil
	} else {
		return "", errors.New("Error: sercret key not match")
	}
}

func (m *UserManager) Users() map[string]*User {
	return m.users
}

/*
* Update the secret key with validation
*/
func (m *UserManager) UpdateSecretKey(validation string, newKey string) error {
	if validation == m.secretKey {
		m.mutex.Lock()
		m.secretKey = newKey
		m.mutex.Unlock()

		return nil
	} else {
		return errors.New("Error: sercret key not match")
	}
}

/*
* Get a registered user by passing the checkCode returned by RegisterUser
*/
func (m *UserManager) Validate(checkCode string) (string, error) {
	if u, ok := m.users[checkCode]; ok {
		if !u.disabled {
			return u.id, nil
		} else {
			return "", errors.New("user expired")
		}
	} else {
		return "", errors.New("no user matched")
	}
}

func (m *UserManager) StartExpireCheck(d time.Duration) {
	if m.ticker != nil {
		m.ticker.Stop()
	}

	m.ticker = time.NewTicker(d)
	go m.ExpireCheck()
}

/*
* Expire check
*/
func (m *UserManager) ExpireCheck() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	for {
		_, open := <-m.ticker.C

		if !open {
			break
		}

		expired := make([]*User, 0, 100)

		for _, u := range m.users {
			if time.Now().Sub(u.createTime) > u.expireTime {
				u.Invalidate()
				expired = append(expired, u)
			}
		}

		m.mutex.Lock()
		for _, u := range expired {
			delete(m.users, u.checkCode)
		}
		m.mutex.Unlock()
	}
}


/*
* Post request should obey the following format:
* -------------Headers---------------
* "User-Id":"xxxx"
* "Secret-Key":"xxxxx"
* "Expire-Time":"xxxxxx"    //minite
*
*
* Return data format is:
* stateCode;checkCode
*
* eg1:ok;xxxxxxxxx
* eg2:error;xxxxxxxx
*/
func (m *UserManager) ServeRegister(w http.ResponseWriter, r *http.Request) {
	if expire, ok := r.Header["Expire-Time"]; ok {
		var expireTime time.Duration

		n, err := strconv.Atoi(expire[0])
		if err != nil {
			w.Write([]byte("error"))
		} else {
			expireTime = time.Minute * time.Duration(n)
		}

		if userId, ok := r.Header["User-Id"]; ok {
			if secretKey, ok := r.Header["Secret-Key"]; ok {
				if checkCode, err := m.RegisterUser(secretKey[0], userId[0], expireTime); err == nil {
					w.Write([]byte("ok;" + checkCode))
				} else {
					w.Write([]byte("error;"))
				}
			} else {
				w.Write([]byte("error;"))
			}
		} else {
			w.Write([]byte("error;"))
		}
	}
}


/*
* Post request should obey the following format:
* --------------Headers-------------------
* "Secret-Key":"xxxxx"
* "New-Key":"xxxxxxx"
*
*
*
* Return format:
* stateCode;
*/
func (m *UserManager) ServeUpdateKey(w http.ResponseWriter, r *http.Request) {
	if secretKey, ok := r.Header["Secret-Key"]; ok {
		if newKey, ok := r.Header["New-Key"]; ok {
			if err := m.UpdateSecretKey(secretKey[0], newKey[0]); err == nil {
				w.Write([]byte("ok;"))
			} else { w.Write([]byte("error;")) }
		} else { w.Write([]byte("error;")) }
	} else { w.Write([]byte("error;")) }
}