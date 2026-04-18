package db

import "strings"

// Migrate creates all required tables and runs additive ALTER TABLE migrations.
// ALTER TABLE errors for columns that already exist are silently ignored so
// that the function is safe to call on both fresh and existing databases.
func (d *DB) Migrate() error {
	var queries []string
	if d.IsPostgres() {
		queries = postgresMigrations
	} else {
		queries = sqliteMigrations
	}
	for _, q := range queries {
		if _, err := d.DB.Exec(q); err != nil {
			// Gracefully ignore "column already exists" when running ALTER TABLE
			// against a database that was created before these columns were added.
			trimmed := strings.TrimSpace(strings.ToUpper(q))
			if strings.HasPrefix(trimmed, "ALTER TABLE") {
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "duplicate column") ||
					strings.Contains(msg, "already exists") ||
					strings.Contains(msg, "column already exists") {
					continue
				}
			}
			return err
		}
	}
	return nil
}

// sqliteMigrations uses SQLite-specific types (AUTOINCREMENT, DATETIME, INTEGER
// for booleans).
var sqliteMigrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		score INTEGER NOT NULL DEFAULT 0,
		affiliation TEXT NOT NULL DEFAULT '',
		website TEXT NOT NULL DEFAULT '',
		country TEXT NOT NULL DEFAULT '',
		banned INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS teams (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		invite_code TEXT UNIQUE NOT NULL,
		score INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS team_members (
		user_id INTEGER NOT NULL,
		team_id INTEGER NOT NULL,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, team_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT NOT NULL,
		category TEXT NOT NULL,
		points INTEGER NOT NULL DEFAULT 100,
		flag TEXT NOT NULL,
		flag_type TEXT NOT NULL DEFAULT 'static',
		deploy_type TEXT NOT NULL DEFAULT 'no_deploy',
		deploy_backend TEXT NOT NULL DEFAULT '',
		deploy_config TEXT NOT NULL DEFAULT '{}',
		image TEXT NOT NULL DEFAULT '',
		vm_template INTEGER NOT NULL DEFAULT 0,
		is_visible INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS submissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		team_id INTEGER,
		challenge_id INTEGER NOT NULL,
		flag TEXT NOT NULL,
		is_correct INTEGER NOT NULL DEFAULT 0,
		ip TEXT NOT NULL DEFAULT '',
		submitted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS instances (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		user_id INTEGER,
		team_id INTEGER,
		instance_type TEXT NOT NULL,
		backend TEXT NOT NULL,
		instance_id TEXT NOT NULL,
		connection_info TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME,
		status TEXT NOT NULL DEFAULT 'running',
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_flags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		team_id INTEGER NOT NULL,
		flag TEXT NOT NULL,
		round INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS ad_services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		team_id INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'up',
		round INTEGER NOT NULL DEFAULT 0,
		score INTEGER NOT NULL DEFAULT 0,
		checked_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS ad_sploits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		team_id INTEGER NOT NULL,
		challenge_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		language TEXT NOT NULL DEFAULT 'python3',
		script TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_run_at DATETIME,
		FOREIGN KEY (team_id) REFERENCES teams(id),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_sploit_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sploit_id INTEGER NOT NULL,
		target_team_id INTEGER NOT NULL,
		round INTEGER NOT NULL,
		stdout TEXT NOT NULL DEFAULT '',
		flags_captured INTEGER NOT NULL DEFAULT 0,
		flags_submitted INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		ran_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (sploit_id) REFERENCES ad_sploits(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_vpn_peers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		team_id INTEGER NOT NULL UNIQUE,
		private_key TEXT NOT NULL,
		public_key TEXT NOT NULL,
		allowed_ip TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_rounds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		round INTEGER NOT NULL UNIQUE,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		created_by INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		draft INTEGER NOT NULL DEFAULT 0,
		auth_required INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_flags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'exact',
		data TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		location TEXT NOT NULL,
		size INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_hints (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		cost INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		tag TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS user_fields (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		field_type TEXT NOT NULL DEFAULT 'text',
		required INTEGER NOT NULL DEFAULT 0,
		public INTEGER NOT NULL DEFAULT 1,
		description TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS user_field_values (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		field_id INTEGER NOT NULL,
		value TEXT NOT NULL DEFAULT '',
		UNIQUE(user_id, field_id)
	)`,
	`CREATE TABLE IF NOT EXISTS ctf_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`,
	// Additive migrations for existing SQLite databases.
	// ALTER TABLE errors for already-existing columns are ignored by Migrate().
	`ALTER TABLE users ADD COLUMN affiliation TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN website TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN country TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN banned INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE users ADD COLUMN verified INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE users ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE users ADD COLUMN language TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE challenges ADD COLUMN connection_info TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE challenges ADD COLUMN max_attempts INTEGER NOT NULL DEFAULT 0`,
}

// postgresMigrations uses PostgreSQL types (BIGSERIAL, TIMESTAMPTZ, BOOLEAN).
var postgresMigrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		score INTEGER NOT NULL DEFAULT 0,
		affiliation TEXT NOT NULL DEFAULT '',
		website TEXT NOT NULL DEFAULT '',
		country TEXT NOT NULL DEFAULT '',
		banned BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS teams (
		id BIGSERIAL PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		invite_code TEXT UNIQUE NOT NULL,
		score INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS team_members (
		user_id BIGINT NOT NULL,
		team_id BIGINT NOT NULL,
		joined_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (user_id, team_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenges (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT NOT NULL,
		category TEXT NOT NULL,
		points INTEGER NOT NULL DEFAULT 100,
		flag TEXT NOT NULL,
		flag_type TEXT NOT NULL DEFAULT 'static',
		deploy_type TEXT NOT NULL DEFAULT 'no_deploy',
		deploy_backend TEXT NOT NULL DEFAULT '',
		deploy_config TEXT NOT NULL DEFAULT '{}',
		image TEXT NOT NULL DEFAULT '',
		vm_template INTEGER NOT NULL DEFAULT 0,
		is_visible BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS submissions (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		team_id BIGINT,
		challenge_id BIGINT NOT NULL,
		flag TEXT NOT NULL,
		is_correct BOOLEAN NOT NULL DEFAULT FALSE,
		ip TEXT NOT NULL DEFAULT '',
		submitted_at TIMESTAMPTZ DEFAULT NOW(),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS instances (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		user_id BIGINT,
		team_id BIGINT,
		instance_type TEXT NOT NULL,
		backend TEXT NOT NULL,
		instance_id TEXT NOT NULL,
		connection_info TEXT NOT NULL DEFAULT '{}',
		created_at TIMESTAMPTZ DEFAULT NOW(),
		expires_at TIMESTAMPTZ,
		status TEXT NOT NULL DEFAULT 'running',
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_flags (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		team_id BIGINT NOT NULL,
		flag TEXT NOT NULL,
		round INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS ad_services (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		team_id BIGINT NOT NULL,
		status TEXT NOT NULL DEFAULT 'up',
		round INTEGER NOT NULL DEFAULT 0,
		score INTEGER NOT NULL DEFAULT 0,
		checked_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS ad_sploits (
		id BIGSERIAL PRIMARY KEY,
		team_id BIGINT NOT NULL,
		challenge_id BIGINT NOT NULL,
		name TEXT NOT NULL,
		language TEXT NOT NULL DEFAULT 'python3',
		script TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		last_run_at TIMESTAMPTZ,
		FOREIGN KEY (team_id) REFERENCES teams(id),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_sploit_results (
		id BIGSERIAL PRIMARY KEY,
		sploit_id BIGINT NOT NULL,
		target_team_id BIGINT NOT NULL,
		round INTEGER NOT NULL,
		stdout TEXT NOT NULL DEFAULT '',
		flags_captured INTEGER NOT NULL DEFAULT 0,
		flags_submitted INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		ran_at TIMESTAMPTZ DEFAULT NOW(),
		FOREIGN KEY (sploit_id) REFERENCES ad_sploits(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_vpn_peers (
		id BIGSERIAL PRIMARY KEY,
		team_id BIGINT NOT NULL UNIQUE,
		private_key TEXT NOT NULL,
		public_key TEXT NOT NULL,
		allowed_ip TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_rounds (
		id BIGSERIAL PRIMARY KEY,
		round INTEGER NOT NULL UNIQUE,
		started_at TIMESTAMPTZ DEFAULT NOW(),
		finished_at TIMESTAMPTZ
	)`,
	`CREATE TABLE IF NOT EXISTS notifications (
		id BIGSERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		created_by BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS pages (
		id BIGSERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		draft BOOLEAN NOT NULL DEFAULT FALSE,
		auth_required BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_flags (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		content TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'exact',
		data TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_files (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		name TEXT NOT NULL,
		location TEXT NOT NULL,
		size BIGINT NOT NULL DEFAULT 0,
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_hints (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		content TEXT NOT NULL,
		cost INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_tags (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		tag TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS user_fields (
		id BIGSERIAL PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		field_type TEXT NOT NULL DEFAULT 'text',
		required BOOLEAN NOT NULL DEFAULT FALSE,
		public BOOLEAN NOT NULL DEFAULT TRUE,
		description TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS user_field_values (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		field_id BIGINT NOT NULL,
		value TEXT NOT NULL DEFAULT '',
		UNIQUE(user_id, field_id)
	)`,
	`CREATE TABLE IF NOT EXISTS ctf_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`,
	// Additive migrations for existing PostgreSQL databases.
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS affiliation TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS website TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS country TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS banned BOOLEAN NOT NULL DEFAULT FALSE`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS verified BOOLEAN NOT NULL DEFAULT FALSE`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT FALSE`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS language TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE challenges ADD COLUMN IF NOT EXISTS connection_info TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE challenges ADD COLUMN IF NOT EXISTS max_attempts INTEGER NOT NULL DEFAULT 0`,
}
