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
		challenge_type TEXT NOT NULL DEFAULT 'static',
		flag_type TEXT NOT NULL DEFAULT 'static',
		checker_config TEXT NOT NULL DEFAULT '{}',
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
		bucket_key TEXT NOT NULL DEFAULT '',
		awarded_points INTEGER NOT NULL DEFAULT 0,
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
		target_id TEXT NOT NULL DEFAULT '',
		instance_id TEXT NOT NULL,
		connection_info TEXT NOT NULL DEFAULT '{}',
		dynamic_flag TEXT NOT NULL DEFAULT '',
		reserved_cpu_mil INTEGER NOT NULL DEFAULT 0,
		reserved_memory_mb INTEGER NOT NULL DEFAULT 0,
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
		bucket_key TEXT NOT NULL DEFAULT '',
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
		bucket_key TEXT NOT NULL DEFAULT '',
		awarded_points INTEGER NOT NULL DEFAULT 0,
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
		provisioned INTEGER NOT NULL DEFAULT 0,
		last_sync_error TEXT NOT NULL DEFAULT '',
		synced_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_rounds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		round INTEGER NOT NULL UNIQUE,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_restart_votes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		user_id INTEGER,
		team_id INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (challenge_id) REFERENCES challenges(id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_restart_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge_id INTEGER NOT NULL,
		triggered_by_user_id INTEGER,
		trigger_mode TEXT NOT NULL DEFAULT 'vote_threshold',
		result TEXT NOT NULL DEFAULT 'success',
		message TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (challenge_id) REFERENCES challenges(id),
		FOREIGN KEY (triggered_by_user_id) REFERENCES users(id)
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
	`CREATE TABLE IF NOT EXISTS modules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		sort_order INTEGER NOT NULL DEFAULT 0,
		draft INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS module_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		module_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		page_id INTEGER,
		challenge_id INTEGER,
		sort_order INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (module_id) REFERENCES modules(id) ON DELETE CASCADE,
		FOREIGN KEY (page_id) REFERENCES pages(id),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
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
	`CREATE INDEX IF NOT EXISTS idx_submissions_user_challenge_correct ON submissions(user_id, challenge_id, is_correct)`,
	`CREATE INDEX IF NOT EXISTS idx_submissions_challenge_correct ON submissions(challenge_id, is_correct)`,
	`CREATE INDEX IF NOT EXISTS idx_instances_challenge_user_status ON instances(challenge_id, user_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_instances_challenge_team_status ON instances(challenge_id, team_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_pages_slug_draft ON pages(slug, draft)`,
	`CREATE INDEX IF NOT EXISTS idx_pages_draft_auth ON pages(draft, auth_required)`,
	`CREATE INDEX IF NOT EXISTS idx_modules_slug_draft ON modules(slug, draft)`,
	`CREATE INDEX IF NOT EXISTS idx_modules_sort ON modules(sort_order, id)`,
	`CREATE INDEX IF NOT EXISTS idx_module_items_module_sort ON module_items(module_id, sort_order, id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_restart_votes_challenge_user ON challenge_restart_votes(challenge_id, user_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_restart_votes_challenge_team ON challenge_restart_votes(challenge_id, team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_restart_events_challenge_created ON challenge_restart_events(challenge_id, created_at)`,
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
	`ALTER TABLE challenges ADD COLUMN challenge_type TEXT NOT NULL DEFAULT 'static'`,
	`ALTER TABLE challenges ADD COLUMN checker_config TEXT NOT NULL DEFAULT '{}'`,
	`ALTER TABLE instances ADD COLUMN dynamic_flag TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE instances ADD COLUMN target_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE instances ADD COLUMN reserved_cpu_mil INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE instances ADD COLUMN reserved_memory_mb INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE submissions ADD COLUMN bucket_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE submissions ADD COLUMN awarded_points INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE ad_sploits ADD COLUMN bucket_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE ad_sploit_results ADD COLUMN bucket_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE ad_sploit_results ADD COLUMN awarded_points INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE ad_vpn_peers ADD COLUMN provisioned INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE ad_vpn_peers ADD COLUMN last_sync_error TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE ad_vpn_peers ADD COLUMN synced_at DATETIME`,
	`ALTER TABLE module_items ADD COLUMN content TEXT NOT NULL DEFAULT ''`,
	// Per-challenge AD rounds
	`ALTER TABLE ad_rounds ADD COLUMN challenge_id INTEGER NOT NULL DEFAULT 0`,
	// Per-signature exploit results on ad_services
	`ALTER TABLE ad_services ADD COLUMN exploit_results TEXT NOT NULL DEFAULT ''`,
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
		challenge_type TEXT NOT NULL DEFAULT 'static',
		flag_type TEXT NOT NULL DEFAULT 'static',
		checker_config TEXT NOT NULL DEFAULT '{}',
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
		bucket_key TEXT NOT NULL DEFAULT '',
		awarded_points INTEGER NOT NULL DEFAULT 0,
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
		target_id TEXT NOT NULL DEFAULT '',
		instance_id TEXT NOT NULL,
		connection_info TEXT NOT NULL DEFAULT '{}',
		dynamic_flag TEXT NOT NULL DEFAULT '',
		reserved_cpu_mil INTEGER NOT NULL DEFAULT 0,
		reserved_memory_mb INTEGER NOT NULL DEFAULT 0,
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
		bucket_key TEXT NOT NULL DEFAULT '',
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
		bucket_key TEXT NOT NULL DEFAULT '',
		awarded_points INTEGER NOT NULL DEFAULT 0,
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
		provisioned BOOLEAN NOT NULL DEFAULT FALSE,
		last_sync_error TEXT NOT NULL DEFAULT '',
		synced_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS ad_rounds (
		id BIGSERIAL PRIMARY KEY,
		round INTEGER NOT NULL UNIQUE,
		started_at TIMESTAMPTZ DEFAULT NOW(),
		finished_at TIMESTAMPTZ
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_restart_votes (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		user_id BIGINT,
		team_id BIGINT,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (team_id) REFERENCES teams(id)
	)`,
	`CREATE TABLE IF NOT EXISTS challenge_restart_events (
		id BIGSERIAL PRIMARY KEY,
		challenge_id BIGINT NOT NULL,
		triggered_by_user_id BIGINT,
		trigger_mode TEXT NOT NULL DEFAULT 'vote_threshold',
		result TEXT NOT NULL DEFAULT 'success',
		message TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ DEFAULT NOW(),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id),
		FOREIGN KEY (triggered_by_user_id) REFERENCES users(id)
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
	`CREATE TABLE IF NOT EXISTS modules (
		id BIGSERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		sort_order INTEGER NOT NULL DEFAULT 0,
		draft BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS module_items (
		id BIGSERIAL PRIMARY KEY,
		module_id BIGINT NOT NULL,
		type TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		page_id BIGINT,
		challenge_id BIGINT,
		sort_order INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (module_id) REFERENCES modules(id) ON DELETE CASCADE,
		FOREIGN KEY (page_id) REFERENCES pages(id),
		FOREIGN KEY (challenge_id) REFERENCES challenges(id)
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
	`CREATE INDEX IF NOT EXISTS idx_submissions_user_challenge_correct ON submissions(user_id, challenge_id, is_correct)`,
	`CREATE INDEX IF NOT EXISTS idx_submissions_challenge_correct ON submissions(challenge_id, is_correct)`,
	`CREATE INDEX IF NOT EXISTS idx_instances_challenge_user_status ON instances(challenge_id, user_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_instances_challenge_team_status ON instances(challenge_id, team_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_pages_slug_draft ON pages(slug, draft)`,
	`CREATE INDEX IF NOT EXISTS idx_pages_draft_auth ON pages(draft, auth_required)`,
	`CREATE INDEX IF NOT EXISTS idx_modules_slug_draft ON modules(slug, draft)`,
	`CREATE INDEX IF NOT EXISTS idx_modules_sort ON modules(sort_order, id)`,
	`CREATE INDEX IF NOT EXISTS idx_module_items_module_sort ON module_items(module_id, sort_order, id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_restart_votes_challenge_user ON challenge_restart_votes(challenge_id, user_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_restart_votes_challenge_team ON challenge_restart_votes(challenge_id, team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_restart_events_challenge_created ON challenge_restart_events(challenge_id, created_at)`,
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
	`ALTER TABLE challenges ADD COLUMN IF NOT EXISTS challenge_type TEXT NOT NULL DEFAULT 'static'`,
	`ALTER TABLE challenges ADD COLUMN IF NOT EXISTS checker_config TEXT NOT NULL DEFAULT '{}'`,
	`ALTER TABLE instances ADD COLUMN IF NOT EXISTS dynamic_flag TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE instances ADD COLUMN IF NOT EXISTS target_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE instances ADD COLUMN IF NOT EXISTS reserved_cpu_mil INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE instances ADD COLUMN IF NOT EXISTS reserved_memory_mb INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE submissions ADD COLUMN IF NOT EXISTS bucket_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE submissions ADD COLUMN IF NOT EXISTS awarded_points INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE ad_sploits ADD COLUMN IF NOT EXISTS bucket_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE ad_sploit_results ADD COLUMN IF NOT EXISTS bucket_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE ad_sploit_results ADD COLUMN IF NOT EXISTS awarded_points INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE ad_vpn_peers ADD COLUMN IF NOT EXISTS provisioned BOOLEAN NOT NULL DEFAULT FALSE`,
	`ALTER TABLE ad_vpn_peers ADD COLUMN IF NOT EXISTS last_sync_error TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE ad_vpn_peers ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ`,
	`ALTER TABLE module_items ADD COLUMN IF NOT EXISTS content TEXT NOT NULL DEFAULT ''`,
	// Per-challenge AD rounds
	`ALTER TABLE ad_rounds ADD COLUMN IF NOT EXISTS challenge_id BIGINT NOT NULL DEFAULT 0`,
	// Per-signature exploit results on ad_services
	`ALTER TABLE ad_services ADD COLUMN IF NOT EXISTS exploit_results TEXT NOT NULL DEFAULT ''`,
}
