package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/walf443/stopwatch"
)

var (
	ErrBannedIP      = errors.New("Banned IP")
	ErrLockedUser    = errors.New("Locked user")
	ErrUserNotFound  = errors.New("Not found user")
	ErrWrongPassword = errors.New("Wrong password")
)

func createLoginLog(succeeded bool, remoteAddr, login string, user *User) error {
	succ := 0
	if succeeded {
		succ = 1
	}

	var userId sql.NullInt64
	if user != nil {
		userId.Int64 = int64(user.ID)
		userId.Valid = true
	}

	_, err := db.Exec(
		"INSERT INTO login_log (`created_at`, `user_id`, `login`, `ip`, `succeeded`) "+
			"VALUES (?,?,?,?,?)",
		time.Now(), userId, login, remoteAddr, succ,
	)

	if succeeded {
		setFailureCount(remoteAddr, 0)
		setFailureCountByUser(user.ID, 0)
	} else {
		incrementFailureCount(remoteAddr)
		incrementFailureCountByUser(user.ID)
	}

	return err
}

func isLockedUser(user *User) (bool, error) {
	if user == nil {
		return false, nil
	}
	var err error

	cnt, found := getFailureCountByUser(user.ID)
	if !found {
		cnt, err = getFailureCountUserFromDB(user.ID)
		if err != nil {
			return false, err
		}
		setFailureCountByUser(user.ID, cnt)
	}

	return UserLockThreshold <= cnt, nil
}

func isBannedIP(ip string) (bool, error) {
	var err error
	cnt, found := getFailureCount(ip)
	if !found {
		cnt, err = getFailureCountFromDB(ip)
		if err != nil {
			return false, err
		}
		setFailureCount(ip, cnt)
	}

	return IPBanThreshold <= cnt, nil
}

func getFailureCountFromDB(ip string) (int, error) {
	var ni sql.NullInt64
	row := db.QueryRow(
		"SELECT COUNT(1) AS failures FROM login_log WHERE "+
			"ip = ? AND id > IFNULL((select id from login_log where ip = ? AND "+
			"succeeded = 1 ORDER BY id DESC LIMIT 1), 0);",
		ip, ip,
	)
	err := row.Scan(&ni)

	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	}

	return int(ni.Int64), nil
}

func getFailureCountUserFromDB(user int) (int, error) {
	var ni sql.NullInt64
	row := db.QueryRow(
		"SELECT COUNT(1) AS failures FROM login_log WHERE "+
			"user_id = ? AND id > IFNULL((select id from login_log where user_id = ? AND "+
			"succeeded = 1 ORDER BY id DESC LIMIT 1), 0);",
		user, user,
	)
	err := row.Scan(&ni)

	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	}

	return int(ni.Int64), nil
}
func getFailureCount(ip string) (int, bool) {
	key := fmt.Sprintf("failure-ip-%s", ip)
	val, found := gocache.Get(key)
	if !found {
		return 0, false
	}
	return val.(int), true
}

func setFailureCount(ip string, cnt int) {
	key := fmt.Sprintf("failure-ip-%s", ip)
	gocache.Set(key, cnt, -1)
}

func incrementFailureCount(ip string) {
	key := fmt.Sprintf("failure-ip-%s", ip)
	err := gocache.Increment(key, 1)
	if err != nil {
		gocache.Set(key, 1, -1)
	}
}


func getFailureCountByUser(user int) (int, bool) {
	key := fmt.Sprintf("failure-user-%d", user)
	val, found := gocache.Get(key)
	if !found {
		return 0, false
	}
	return val.(int), true
}

func setFailureCountByUser(user int, cnt int) {
	key := fmt.Sprintf("failure-user-%d", user)
	gocache.Set(key, cnt, -1)
}

func incrementFailureCountByUser(user int) {
	key := fmt.Sprintf("failure-user-%d", user)
	err := gocache.Increment(key, 1)
	if err != nil {
		gocache.Set(key, 1, -1)
	}
}

func attemptLogin(req *http.Request) (*User, error) {
	succeeded := false
	user := &User{}

	loginName := req.PostFormValue("login")
	password := req.PostFormValue("password")

	remoteAddr := req.RemoteAddr
	if xForwardedFor := req.Header.Get("X-Forwarded-For"); len(xForwardedFor) > 0 {
		remoteAddr = xForwardedFor
	}

	defer func() {
		createLoginLog(succeeded, remoteAddr, loginName, user)
	}()

	stopwatch.Watch("before db query")
	row := db.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE login = ?",
		loginName,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)
	stopwatch.Watch("after db query")

	switch {
	case err == sql.ErrNoRows:
		user = nil
	case err != nil:
		return nil, err
	}

	if banned, _ := isBannedIP(remoteAddr); banned {
		return nil, ErrBannedIP
	}
	stopwatch.Watch("after isBannedIP")

	if locked, _ := isLockedUser(user); locked {
		return nil, ErrLockedUser
	}
	stopwatch.Watch("after isLockedUser")

	if user == nil {
		return nil, ErrUserNotFound
	}

	if user.PasswordHash != calcPassHash(password, user.Salt) {
		return nil, ErrWrongPassword
	}
	stopwatch.Watch("after calcPassHash")

	succeeded = true
	return user, nil
}

func getCurrentUser(userId interface{}) *User {
	user := &User{}
	row := db.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE id = ?",
		userId,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)

	if err != nil {
		return nil
	}

	return user
}

func bannedIPs() []string {
	ips := []string{}

	rows, err := db.Query(
		"SELECT ip FROM "+
			"(SELECT ip, MAX(succeeded) as max_succeeded, COUNT(1) as cnt FROM login_log GROUP BY ip) "+
			"AS t0 WHERE t0.max_succeeded = 0 AND t0.cnt >= ?",
		IPBanThreshold,
	)

	if err != nil {
		return ips
	}

	defer rows.Close()
	for rows.Next() {
		var ip string

		if err := rows.Scan(&ip); err != nil {
			return ips
		}
		ips = append(ips, ip)
	}
	if err := rows.Err(); err != nil {
		return ips
	}

	rowsB, err := db.Query(
		"SELECT ip, MAX(id) AS last_login_id FROM login_log WHERE succeeded = 1 GROUP by ip",
	)

	if err != nil {
		return ips
	}

	defer rowsB.Close()
	for rowsB.Next() {
		var ip string
		var lastLoginId int

		if err := rows.Scan(&ip, &lastLoginId); err != nil {
			return ips
		}

		var count int

		err = db.QueryRow(
			"SELECT COUNT(1) AS cnt FROM login_log WHERE ip = ? AND ? < id",
			ip, lastLoginId,
		).Scan(&count)

		if err != nil {
			return ips
		}

		if IPBanThreshold <= count {
			ips = append(ips, ip)
		}
	}
	if err := rowsB.Err(); err != nil {
		return ips
	}

	return ips
}

func lockedUsers() []string {
	userIds := []string{}

	rows, err := db.Query(
		"SELECT user_id, login FROM "+
			"(SELECT user_id, login, MAX(succeeded) as max_succeeded, COUNT(1) as cnt FROM login_log GROUP BY user_id) "+
			"AS t0 WHERE t0.user_id IS NOT NULL AND t0.max_succeeded = 0 AND t0.cnt >= ?",
		UserLockThreshold,
	)

	if err != nil {
		return userIds
	}

	defer rows.Close()
	for rows.Next() {
		var userId int
		var login string

		if err := rows.Scan(&userId, &login); err != nil {
			return userIds
		}
		userIds = append(userIds, login)
	}
	if err := rows.Err(); err != nil {
		return userIds
	}

	rowsB, err := db.Query(
		"SELECT user_id, login, MAX(id) AS last_login_id FROM login_log WHERE user_id IS NOT NULL AND succeeded = 1 GROUP BY user_id",
	)

	if err != nil {
		return userIds
	}

	defer rowsB.Close()
	for rowsB.Next() {
		var userId int
		var login string
		var lastLoginId int

		if err := rowsB.Scan(&userId, &login, &lastLoginId); err != nil {
			return userIds
		}

		var count int

		err = db.QueryRow(
			"SELECT COUNT(1) AS cnt FROM login_log WHERE user_id = ? AND ? < id",
			userId, lastLoginId,
		).Scan(&count)

		if err != nil {
			return userIds
		}

		if UserLockThreshold <= count {
			userIds = append(userIds, login)
		}
	}
	if err := rowsB.Err(); err != nil {
		return userIds
	}

	return userIds
}
