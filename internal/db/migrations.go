package db

func (d *DB) Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			score INTEGER NOT NULL DEFAULT 0,
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
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
