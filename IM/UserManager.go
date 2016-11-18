package IM

import (
	"sync"
	"crypto/md5"
	"time"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"io/ioutil"
	"strings"
	"log"
)

const (
	DefaultReceive = iota
	DefaultReject
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

	userFilter UserFilter
}
func (u* User) Invalidate() {
	u.mutex.Lock()
	u.disabled = true
	u.mutex.Unlock()
}
func (u *User) ParseParams(params string) {
	ps := strings.Split(params, "\n")
	for _, p := range ps {
		index := strings.Index(p, ":")

		if index == -1 {
			continue
		}

		key := p[:index]
		content := p[index + 1:]

		switch key {
		case "RL":
			us := strings.Split(content, ";")
			u.userFilter.AddToReceiveList(us)
			break
		case "BL":
			us := strings.Split(content, ";")
			u.userFilter.AddToBlackList(us)
			break
		default:
		}
	}
}
func NewUser(id string, checkCode string, expireTime time.Duration, recMode uint8, params... string) *User {
	u := &User{
		id		: id,
		checkCode	: checkCode,
		expireTime	: expireTime,

		userFilter	: UserFilter{
			receiveList	: make(map[string]uint8),
			blackList	: make(map[string]uint8),
			recMode		: recMode,
		},
	}

	if len(params) > 0 {
		u.ParseParams(params[0])
	}

	return u
}

type UserFilter struct {
	recMode uint8
	blackList map[string]uint8
	receiveList map[string]uint8
}

func (f *UserFilter) AddToReceiveList(users []string) {
	for _, u := range users {
		f.receiveList[u] = 0
	}
}
func (f *UserFilter) RemoveFromReceiveList(users []string) {
	for _, u := range users {
		delete(f.receiveList, u)
	}
}
func (f *UserFilter) AddToBlackList(users []string) {
	for _, u := range users {
		f.blackList[u] = 0
	}
}
func (f *UserFilter) RemoveFromBlackList(users []string) {
	for _, u := range users {
		delete(f.blackList, u)
	}
}
func (f *UserFilter) IsReceived(id string) bool {
	for s := range f.blackList {
		if s == id {
			return false
		}
	}
	for s := range f.receiveList {
		if s == id {
			return true
		}
	}

	if f.recMode == DefaultReceive {
		return true
	} else {
		return false
	}
}
func (f *UserFilter) Filter(src_ms []Message) []Message {
	res_ms := make([]Message, 0, 5)
	for _, m := range src_ms {
		sid := m.SenderId()
		if f.IsReceived(sid) {
			res_ms = append(res_ms, m)
		}
	}

	return res_ms
}

func NewCheckCode(id string) (string, error) {
	h := md5.New()
	h.Write([]byte(id + time.Now().String()))
	return hex.EncodeToString(h.Sum(nil)), nil
}

/*
* Register a user with user's id and the secret key
*/
func (m *UserManager) RegisterUser(validation string, id string, expireTime time.Duration, recMode uint8, params... string) (string, error) {
	m.mutex.RLock()
	sk := m.secretKey
	m.mutex.RUnlock()

	if validation == sk {
		checkCode, err := NewCheckCode(id)
		if err != nil {
			log.Print(err)
			return "", err
		}

		m.mutex.Lock()
		m.users[checkCode] = NewUser(id, checkCode, expireTime, recMode, params...)
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
func (m *UserManager) Validate(checkCode string) (*User, error) {
	if u, ok := m.users[checkCode]; ok {
		if !u.disabled {
			return u, nil
		} else {
			return nil, errors.New("user expired")
		}
	} else {
		return nil, errors.New("no user matched")
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
			log.Print(err)
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
* "Received-Mode":"xxxxx"   //choices : "DefaultReceive" 、 "DefaultReject"
* "Content-Type":"text/plain"
* -------------Content----------------
* BL:user1;user2;...;		//black list
* RL:user1;user2;...;		//receive list
*
* Return data format is:
* stateCode;checkCode
*
* eg1:ok;xxxxxxxxx
* eg2:error;xxxxxxxx
*/
func (m *UserManager) ServeRegister(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

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
				var recMode uint8
				if rec, ok := r.Header["Receive-Mode"]; ok {
					switch rec[0] {
					case "DefaultReceive":
						recMode = DefaultReceive
						break
					case "DefaultReject":
						recMode = DefaultReject
						break
					default:
						recMode = DefaultReceive
					}
				} else {
					recMode = DefaultReceive
				}

				body, _ := ioutil.ReadAll(r.Body)
				log.Print(string(body))
				if checkCode, err := m.RegisterUser(secretKey[0], userId[0], expireTime, recMode, string(body)); err == nil {
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
* Put request should obey the following format:
* ----------------Headers-----------------------
* "User-CheckCode":"xxxxx"
* "List":"xxxxx"			//list can be "RL" "BL"
* ----------------Content-----------------------
* Add u1;u2				//Add users to the list
* Del u1;u2				//Del users from the list
*/
func (m *UserManager) ServeUpdateReceiveList(w http.ResponseWriter, r * http.Request) {
	defer r.Body.Close()

	if check, ok := r.Header["User-CheckCode"]; ok {
		if list, ok := r.Header["List"]; ok && (list[0] == "RL" || list[0] == "BL") {
			body, _ := ioutil.ReadAll(r.Body)

			cmds := strings.Split(string(body), "\n")
			for _, cmd := range cmds {
				parts := strings.Split(cmd, " ")
				if len(parts) < 2 {
					continue
				}

				switch parts[0] {
				case "Add":
					us := strings.Split(parts[1], ";")
					if list[0] == "RL" {
						m.users[check[0]].userFilter.AddToReceiveList(us)
					} else {
						m.users[check[0]].userFilter.AddToBlackList(us)
					}
					break
				case "Del":
					us := strings.Split(parts[1], ";")
					if list[0] == "RL" {
						m.users[check[0]].userFilter.RemoveFromReceiveList(us)
					} else {
						m.users[check[0]].userFilter.RemoveFromBlackList(us)
					}
					break
				default:
				}
			}
		} else { log.Print("update list not set") }
	} else { log.Print("user not set") }
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

