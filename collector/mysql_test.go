package collector

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"weops-inspect/config"
)

func TestClassifyMySQLAuth(t *testing.T) {
	cases := []struct {
		name string
		num  uint16
	}{
		{"access denied", 1045},
		{"db access denied", 1044},
		{"native auth failed", 1698},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := &mysql.MySQLError{Number: c.num, Message: "Access denied"}
			if got := classifyMySQL(err); got != ErrAuth {
				t.Fatalf("MySQLError %d should be ErrAuth, got %q", c.num, got)
			}
		})
	}
}

func TestClassifyMySQLProtocol(t *testing.T) {
	err := &mysql.MySQLError{Number: 1064, Message: "syntax error"}
	if got := classifyMySQL(err); got != ErrProtocol {
		t.Fatalf("non-auth MySQLError should be ErrProtocol, got %q", got)
	}
}

func TestClassifyMySQLFallsBackToGeneric(t *testing.T) {
	err := errors.New("dial tcp 1.2.3.4:3306: connection refused")
	if got := classifyMySQL(err); got != ErrNetwork {
		t.Fatalf("non-MySQLError should fall back to Classify(network), got %q", got)
	}
}

// pickFirst 必须按列名取值,且对 5.7 / 8.4 字段重命名提供等价回退。
func TestPickFirstColumnRename(t *testing.T) {
	// MySQL 5.7 风格列。
	cols57 := []string{"Slave_IO_Running", "Slave_SQL_Running"}
	row57 := []sql.RawBytes{[]byte("Yes"), []byte("Yes")}
	if v := pickFirst(cols57, row57, "Slave_IO_Running", "Replica_IO_Running"); v != "Yes" {
		t.Fatalf("5.7 lookup failed: %q", v)
	}

	// MySQL 8.4 风格列。
	cols84 := []string{"Replica_IO_Running", "Replica_SQL_Running"}
	row84 := []sql.RawBytes{[]byte("Yes"), []byte("No")}
	if v := pickFirst(cols84, row84, "Slave_IO_Running", "Replica_IO_Running"); v != "Yes" {
		t.Fatalf("8.4 lookup failed (IO): %q", v)
	}
	if v := pickFirst(cols84, row84, "Slave_SQL_Running", "Replica_SQL_Running"); v != "No" {
		t.Fatalf("8.4 lookup failed (SQL): %q", v)
	}

	// 不存在的列。
	if v := pickFirst(cols84, row84, "Bogus_Col"); v != "" {
		t.Fatalf("missing col should return empty, got %q", v)
	}
}

// 确保 ctx 截止能切断 query。给一个不可达的本地端口。
func TestOpenMySQLContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	db, err := openMySQL("127.0.0.1", "1", config.Credentials{MySQLUser: "u", MySQLPassword: "p"})
	if err != nil {
		t.Fatalf("open should not fail synchronously: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err == nil {
		t.Fatalf("ping should fail against unreachable port")
	}
}
