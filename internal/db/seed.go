package db

// SeedChallenge holds the data for a sample challenge to be inserted.
type SeedChallenge struct {
	Name        string
	Description string
	Category    string
	Points      int
	Flag        string
	FlagType    string
	DeployType  string
}

// defaultChallenges is the built-in set of starter CTF tasks covering common
// categories. All tasks use the no_deploy type — the flags are embedded here
// and can be replaced by admins after first run.
var defaultChallenges = []SeedChallenge{
	// ── Misc ────────────────────────────────────────────────────────────────
	{
		Name:     "Sanity Check",
		Category: "misc",
		Points:   1,
		Flag:     "FLAG{w3lc0m3_t0_th3_ctf}",
		FlagType: "static",
		Description: `Welcome to the CTF!

The flag for this challenge is hidden in plain sight.
Read the rules page carefully — or just submit: **FLAG{w3lc0m3_t0_th3_ctf}**

This task is worth 1 point and is here so you can verify that flag submission works.`,
		DeployType: "no_deploy",
	},
	{
		Name:     "Discord Treasure",
		Category: "misc",
		Points:   10,
		Flag:     "FLAG{y0u_f0und_th3_d1sc0rd_fl4g}",
		FlagType: "static",
		Description: `Join our Discord server and look for the flag hidden in one of the channels.

**Hint:** Check the #announcements channel pinned messages.`,
		DeployType: "no_deploy",
	},

	// ── Cryptography ────────────────────────────────────────────────────────
	{
		Name:     "ROT13 Warmup",
		Category: "crypto",
		Points:   50,
		Flag:     "FLAG{r0t13_1s_n0t_3ncrypt10n}",
		FlagType: "static",
		Description: `ROT13 is a classic substitution cipher that replaces each letter with the
letter 13 positions after it in the alphabet.

Decode the following ciphertext to get the flag:

` + "```\nSYNT{e0g13_1f_a0g_3apelcg10a}\n```" + `

**Hint:** ROT13 is its own inverse — applying it twice returns the original text.`,
		DeployType: "no_deploy",
	},
	{
		Name:     "Base64 Basics",
		Category: "crypto",
		Points:   50,
		Flag:     "FLAG{b4s364_1s_n0t_s3cur1ty}",
		FlagType: "static",
		Description: `Base64 is an encoding scheme (NOT encryption) commonly seen in web tokens
and email attachments.

Decode the following string:

` + "```\nRkxBR3tiNHMzNjRfMXNfbjB0X3MzY3VyMXR5fQ==\n```" + `

**Tools:** ` + "`" + `echo 'ENCODED' | base64 -d` + "`" + ` or https://cyberchef.io`,
		DeployType: "no_deploy",
	},
	{
		Name:     "Caesar's Secret",
		Category: "crypto",
		Points:   75,
		Flag:     "FLAG{v3ni_v1d1_v1c1}",
		FlagType: "static",
		Description: `Julius Caesar used a simple substitution cipher to protect his messages.
Each letter is shifted by a fixed number of positions in the alphabet.

Ciphertext (shift unknown, try all 25):

` + "```\nIOCQ{y3pl_y1h1_y1f1}\n```" + `

**Note:** The FLAG{ prefix will also be shifted. Find the shift that makes the
prefix read FLAG{.`,
		DeployType: "no_deploy",
	},
	{
		Name:     "XOR Encryption",
		Category: "crypto",
		Points:   150,
		Flag:     "FLAG{xor_1s_r3v3rs1bl3}",
		FlagType: "static",
		Description: `XOR is a bitwise operation frequently used in basic encryption.
Given the key and ciphertext (hex-encoded), recover the plaintext flag.

` + "```\nKey (hex):        0x42\nCiphertext (hex): 16 2d 17 13 7b 33 39 30 7a 33 76 33 72 73 31 62 6c 33 7d\n```" + `

**Hint:** XOR each byte of the ciphertext with the key byte.  
**Tools:** Python ` + "`" + `bytes([b ^ 0x42 for b in bytes.fromhex('...')])` + "`",
		DeployType: "no_deploy",
	},

	// ── Web ──────────────────────────────────────────────────────────────────
	{
		Name:     "Cookie Monster",
		Category: "web",
		Points:   100,
		Flag:     "FLAG{c00k13s_4r3_d3l1c10us}",
		FlagType: "static",
		Description: `The admin of this site forgot to properly protect the admin cookie.

1. Open the challenge URL in your browser.
2. Open DevTools (F12) → Application → Cookies.
3. Change the ` + "`" + `role` + "`" + ` cookie value to ` + "`" + `admin` + "`" + `.
4. Refresh the page.

**Deploy type:** single instance — use the "Start Instance" button to get the URL.`,
		DeployType: "no_deploy",
	},
	{
		Name:     "SQL Injection 101",
		Category: "web",
		Points:   200,
		Flag:     "FLAG{sq1_1nj3ct10n_byp4ss}",
		FlagType: "static",
		Description: `A login form on the target site is vulnerable to classic SQL Injection.

**Goal:** Bypass authentication without knowing the password.

Try entering the following as the username:

` + "```\n' OR '1'='1\n```" + `

**Deploy type:** single instance — click "Start Instance" to get the challenge URL.

**Hint:** The query looks like: ` + "`" + `SELECT * FROM users WHERE username='$input' AND password='$pass'` + "`",
		DeployType: "no_deploy",
	},
	{
		Name:     "Hidden in Headers",
		Category: "web",
		Points:   75,
		Flag:     "FLAG{http_h34d3rs_4r3_1nt3r3st1ng}",
		FlagType: "static",
		Description: `Sometimes developers hide secrets in HTTP response headers.

Use ` + "`" + `curl -I https://example.com` + "`" + ` or browser DevTools (Network tab) to inspect the
response headers from the challenge URL and find the hidden flag.

**Deploy type:** single instance — start the instance to get the URL.`,
		DeployType: "no_deploy",
	},

	// ── Forensics ───────────────────────────────────────────────────────────
	{
		Name:     "Metadata Hunt",
		Category: "forensics",
		Points:   100,
		Flag:     "FLAG{3x1f_d4t4_l34ks}",
		FlagType: "static",
		Description: `Download the attached image and examine its EXIF metadata — the flag is
hiding in one of the metadata fields.

**Tools:**
- ` + "`" + `exiftool image.jpg` + "`" + `
- https://exifdata.com

[Download image](https://example.com/challenge-files/metadata_hunt.jpg)`,
		DeployType: "no_deploy",
	},
	{
		Name:     "LSB Steganography",
		Category: "forensics",
		Points:   200,
		Flag:     "FLAG{1sb_st3g_1s_fun}",
		FlagType: "static",
		Description: `An image has a secret message encoded using Least Significant Bit (LSB)
steganography.

**Tools:**
- [StegOnline](https://stegonline.georgeom.net/upload)
- ` + "`" + `steghide extract -sf image.png` + "`" + `
- Python: ` + "`" + `from PIL import Image` + "`" + ` — read LSBs of R channel

[Download image](https://example.com/challenge-files/lsb_steg.png)`,
		DeployType: "no_deploy",
	},

	// ── Reverse Engineering ──────────────────────────────────────────────────
	{
		Name:     "String Search",
		Category: "reverse",
		Points:   100,
		Flag:     "FLAG{str1ngs_4r3_y0ur_fr13nd}",
		FlagType: "static",
		Description: `A compiled binary has the flag stored as a plaintext string inside it.

Run ` + "`" + `strings binary | grep FLAG` + "`" + ` to extract it.

[Download binary](https://example.com/challenge-files/string_search)

**Tools:** ` + "`" + `strings` + "`" + `, ` + "`" + `grep` + "`" + `, Ghidra, IDA Free`,
		DeployType: "no_deploy",
	},
	{
		Name:     "Anti-Debug",
		Category: "reverse",
		Points:   300,
		Flag:     "FLAG{4nt1_d3bug_byp4ss3d}",
		FlagType: "static",
		Description: `This binary checks if it is being debugged and exits immediately if so.

**Objectives:**
1. Identify the anti-debug check (` + "`" + `ptrace` + "`" + ` or ` + "`" + `IsDebuggerPresent` + "`" + `)
2. Patch the binary or use ` + "`" + `LD_PRELOAD` + "`" + ` to bypass it
3. Extract the flag

[Download binary](https://example.com/challenge-files/anti_debug)

**Tools:** Ghidra, GDB, pwndbg`,
		DeployType: "no_deploy",
	},

	// ── Pwn ──────────────────────────────────────────────────────────────────
	{
		Name:     "Buffer Overflow 101",
		Category: "pwn",
		Points:   200,
		Flag:     "FLAG{buff3r_0v3rfl0w_pwn3d}",
		FlagType: "static",
		Description: `A classic stack-based buffer overflow challenge.

The target binary reads input into a fixed-size buffer without bounds checking.
Overflow the buffer to overwrite the saved return address and redirect execution
to the ` + "`" + `win()` + "`" + ` function.

**Deploy type:** single instance (netcat service)
` + "```bash\nnc <host> <port>\n```" + `

**Tools:** pwntools, GDB + pwndbg, checksec

**Protections:** No canary, No PIE, NX enabled`,
		DeployType: "no_deploy",
	},
	{
		Name:     "Format String Bug",
		Category: "pwn",
		Points:   350,
		Flag:     "FLAG{f0rm4t_str1ng_1s_p0w3rful}",
		FlagType: "static",
		Description: `The binary uses ` + "`" + `printf(user_input)` + "`" + ` directly — a format string vulnerability.

Use format specifiers like ` + "`" + `%x` + "`" + `, ` + "`" + `%s` + "`" + `, ` + "`" + `%n` + "`" + ` to leak stack values and
overwrite a global variable to unlock the flag.

**Deploy type:** single instance (netcat service)
` + "```bash\nnc <host> <port>\n```" + `

**Tools:** pwntools (` + "`" + `fmtstr_payload` + "`" + `), GDB`,
		DeployType: "no_deploy",
	},
}

// Seed inserts the built-in sample challenges if the challenges table is empty.
// It is safe to call on every startup — it will not duplicate rows.
func (d *DB) Seed() error {
	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM challenges`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		// Already seeded (or admin has added their own challenges).
		return nil
	}

	stmt, err := d.Prepare(`
		INSERT INTO challenges
			(name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, is_visible)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, '', '{}', 1)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ch := range defaultChallenges {
		if _, err := stmt.Exec(
			ch.Name, ch.Description, ch.Category,
			ch.Points, ch.Flag, ch.FlagType, ch.DeployType,
		); err != nil {
			return err
		}
	}
	return nil
}
