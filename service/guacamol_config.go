package service

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/easy-cloud-Knet/KWS_Control/util"
	_ "github.com/go-sql-driver/mysql"
)

func GuacamoleConfig(Username string, UUID string, Ip string, PrivateKey string, db *sql.DB) string {
	log := util.GetLogger()

	if db == nil {
		log.Error("guacamole: db connection is nil")
		return ""
	}

	// 1. 무작위 비밀번호 생성
	userPass, err := generateRandomPassword(12)
	if err != nil {
		log.Error("guacamole: failed to generate random password:", err, true)
		return ""
	}
	fmt.Println("생성된 비밀번호:", userPass)

	// 2. 32바이트 Salt 생성
	salt, err := generateRandomSalt(32)
	if err != nil {
		log.Error("guacamole: failed to create random salt:", err, true)
		return ""
	}
	fmt.Println("생성된 salt:", hex.EncodeToString(salt))

	// 3. 해시 계산: SHA256(salt + password)
	passwordHash := fmt.Sprintf("%x", hashPasswordWithSalt(userPass, salt))
	saltHex := fmt.Sprintf("%x", salt)
	tx, err := db.Begin()
	if err != nil {
		log.Error("guacamole: failed to start transaction: %v", err, true)
		return ""
	}

	defer func() {
		if err != nil {
			log.DebugWarn("guacamole: rolling back transaction due to error")
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Error("guacamole: failed to rollback transaction:", rbErr, true)
			}
		}
	}()

	// 4. Entity 생성
	// 중복 확인 이후
	var entityID int64
	var res sql.Result
	err = tx.QueryRow(`SELECT entity_id FROM guacamole_entity WHERE name = ? AND type = 'USER'`, UUID).Scan(&entityID)
	if err == sql.ErrNoRows {
		// Entity가 없으면 새로 생성
		res, err = tx.Exec(`INSERT INTO guacamole_entity (name, type) VALUES (?, 'USER')`, UUID)
		if err != nil {
			log.Error("guacamole: failed to create an entity:", err, true)
			return ""
		}

		entityID, err = res.LastInsertId()
		if err != nil {
			log.Error("guacamole: failed to retrieve entity id:", err, true)
			return ""
		}
		log.DebugInfo("guacamole: created new entity with ID: %d", entityID)
	} else if err != nil {
		log.Error("guacamole: failed to check existing entity:", err, true)
		return ""
	} else {
		// Entity가 이미 존재
		log.DebugInfo("guacamole: using existing entity with ID: %d", entityID)
	}

	// 5. User 생성
	_, err = tx.Exec(`
		INSERT INTO guacamole_user (entity_id, password_hash, password_salt, password_date)
		VALUES (?, UNHEX(?), UNHEX(?), NOW())
		ON DUPLICATE KEY UPDATE
		password_hash = VALUES(password_hash),
		password_salt = VALUES(password_salt),
		password_date = VALUES(password_date)
	`, entityID, passwordHash, saltHex)
	if err != nil {
		log.Error("guacamole: failed to create user:", err, true)
		return ""
	}

	// 6. Connection 생성
	connectionName := fmt.Sprintf("%s-ssh", UUID)
	res, err = tx.Exec(`
		INSERT INTO guacamole_connection (connection_name, protocol)
		VALUES (?, 'ssh')
		ON DUPLICATE KEY UPDATE connection_name = VALUES(connection_name)
	`, connectionName)
	if err != nil {
		log.Error("guacamole: failed to create connection:", err, true)
		return ""
	}

	connectionID, err := res.LastInsertId()
	if err != nil {
		log.Error("guacamole: failed to retrieve connection id:", err, true)
		return ""
	}

	// 7. Connection parameters 설정
	parameters := map[string]string{
		"hostname":    Ip,
		"port":        "22",
		"username":    Username,
		"private-key": PrivateKey,
	}

	for name, value := range parameters {
		_, err = tx.Exec(`
			INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE parameter_value = VALUES(parameter_value)
		`, connectionID, name, value)
		if err != nil {
			log.Error("guacamole: failed to set parameter %s: %v", name, err, true)
			return ""
		}
	}

	// 8. User에게 Connection 권한 부여
	_, err = tx.Exec(`
		INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
		VALUES (?, ?, 'READ')
		ON DUPLICATE KEY UPDATE permission = VALUES(permission)
	`, entityID, connectionID)
	if err != nil {
		log.Error("guacamole: failed to grant permission:", err, true)
		return ""
	}

	// 9. 트랜잭션 커밋
	if err = tx.Commit(); err != nil {
		log.Error("guacamole: failed to commit transaction:", err, true)
		return ""
	}

	return userPass
}

// SHA256 해시 함수 (salt 포함)
func hashPasswordWithSalt(password string, salt []byte) []byte {
	hash := sha256.New()

	var temp = hex.EncodeToString(salt)

	temp = strings.ToUpper(temp)
	hash.Write([]byte(password))
	hash.Write([]byte(temp))
	return hash.Sum(nil)
}

// 랜덤 salt 생성 (32바이트)
func generateRandomSalt(length int) ([]byte, error) {
	salt := make([]byte, length)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}
	return salt, nil
}

// 안전한 랜덤 비밀번호 생성 함수
func generateRandomPassword(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}

// 뭔가뭔가 문제가 생겼을 때, 지우는 무언가
func CleanupGuacamoleConfig(UUID string, db *sql.DB) error {
	log := util.GetLogger()

	if db == nil {
		log.Error("failed to establish database connection: db is nil")
		return fmt.Errorf("db is nil")
	}

	if err := db.Ping(); err != nil {
		log.Error("database connection test failed: %v", err, true)
		return fmt.Errorf("database connection test failed: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Error("failed to start transaction: %v", err, true)
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if err != nil {
			log.DebugWarn("rolling back transaction due to error")
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Error("failed to rollback transaction:", rbErr, true)
			}
		}
	}()

	// Entity ID 찾기
	var entityID int64
	err = tx.QueryRow(`SELECT entity_id FROM guacamole_entity WHERE name = ? AND type = 'USER'`, UUID).Scan(&entityID)
	if err == sql.ErrNoRows {
		log.DebugInfo("no entity found for UUID %s, nothing to clean up", UUID)
		return nil
	} else if err != nil {
		log.Error("failed to find entity for UUID %s: %v", UUID, err, true)
		return fmt.Errorf("failed to find entity for UUID %s: %w", UUID, err)
	}

	// Entity 삭제 // cascade로 user도 같이 삭제
	_, err = tx.Exec(`DELETE FROM guacamole_entity WHERE entity_id = ?`, entityID)
	if err != nil {
		log.Error("failed to delete entity for UUID %s: %v", UUID, err, true)
		return fmt.Errorf("failed to delete entity for UUID %s: %w", UUID, err)
	}

	// UUID와 관련된 orphaned connections 정리
	connectionName := fmt.Sprintf("%s-ssh", UUID)
	_, err = tx.Exec(`
		DELETE FROM guacamole_connection 
		WHERE connection_name = ? AND connection_id NOT IN (
			SELECT DISTINCT connection_id FROM guacamole_connection_permission
		)
	`, connectionName)
	if err != nil {
		log.Error("failed to delete orphaned connections: %v", err, true)
		return fmt.Errorf("failed to delete orphaned connections: %w", err)
	}

	if err = tx.Commit(); err != nil {
		log.Error("failed to commit transaction: %v", err, true)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info("successfully cleaned up configuration for UUID %s", UUID, true)
	return nil
}
