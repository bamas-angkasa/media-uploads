package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/media-share/config"
	"github.com/yourusername/media-share/internal/database"
	"github.com/yourusername/media-share/internal/shortcode"
	"golang.org/x/crypto/bcrypt"
)

type seedUser struct {
	email    string
	username string
	password string
	role     string
}

type seedMedia struct {
	title       string
	description string
	tags        []string
	width       int
	height      int
	fileSize    int64
	// s3Key is used as both original_key and the thumbnail key prefix
	s3Key string
}

var defaultUsers = []seedUser{
	{
		email:    "admin@mediashare.local",
		username: "admin",
		password: "Admin1234!",
		role:     "admin",
	},
	{
		email:    "user@mediashare.local",
		username: "testuser",
		password: "User1234!",
		role:     "user",
	},
}

// seedMediaItems are seeded for "testuser". Tags act as categories.
var seedMediaItems = []seedMedia{
	{
		title:       "Forest Trail at Dawn",
		description: "A peaceful morning hike through a dense forest trail with golden light filtering through the trees.",
		tags:        []string{"nature", "landscape", "forest"},
		width:       1920, height: 1280, fileSize: 2_450_000,
		s3Key: "seed/nature/forest-trail",
	},
	{
		title:       "Mountain Lake Reflection",
		description: "Crystal-clear alpine lake perfectly mirroring the surrounding peaks.",
		tags:        []string{"nature", "landscape", "mountains"},
		width:       2400, height: 1600, fileSize: 3_100_000,
		s3Key: "seed/nature/mountain-lake",
	},
	{
		title:       "Portrait — Golden Hour",
		description: "Studio-style outdoor portrait taken during the golden hour with a shallow depth of field.",
		tags:        []string{"portrait", "people", "photography"},
		width:       1080, height: 1350, fileSize: 1_800_000,
		s3Key: "seed/portrait/golden-hour",
	},
	{
		title:       "Street Candid — Market Day",
		description: "Candid shot of a busy local market capturing the energy of everyday life.",
		tags:        []string{"portrait", "street", "people"},
		width:       1200, height: 800, fileSize: 1_500_000,
		s3Key: "seed/portrait/market-day",
	},
	{
		title:       "Modern Glass Tower",
		description: "Abstract angle of a downtown skyscraper's glass facade reflecting the sky.",
		tags:        []string{"architecture", "urban", "buildings"},
		width:       1600, height: 2400, fileSize: 2_200_000,
		s3Key: "seed/architecture/glass-tower",
	},
	{
		title:       "Historic Stone Bridge",
		description: "An ancient stone bridge spanning a narrow river in rural Europe.",
		tags:        []string{"architecture", "travel", "history"},
		width:       1920, height: 1080, fileSize: 1_900_000,
		s3Key: "seed/architecture/stone-bridge",
	},
	{
		title:       "Red Fox in Snow",
		description: "A wild red fox hunting in fresh snow, captured in a rural woodland.",
		tags:        []string{"animals", "wildlife", "nature"},
		width:       2000, height: 1333, fileSize: 2_800_000,
		s3Key: "seed/animals/red-fox",
	},
	{
		title:       "Tabby Cat Napping",
		description: "Close-up of a tabby cat curled up in a sunny window.",
		tags:        []string{"animals", "pets", "cute"},
		width:       1080, height: 1080, fileSize: 900_000,
		s3Key: "seed/animals/tabby-cat",
	},
	{
		title:       "Artisan Pizza",
		description: "Wood-fired margherita pizza fresh out of the oven with charred crust and fresh basil.",
		tags:        []string{"food", "cooking", "italian"},
		width:       1200, height: 900, fileSize: 1_100_000,
		s3Key: "seed/food/artisan-pizza",
	},
	{
		title:       "Fresh Fruit Bowl",
		description: "Vibrant overhead shot of a bowl filled with seasonal tropical fruits.",
		tags:        []string{"food", "healthy", "colorful"},
		width:       1200, height: 1200, fileSize: 980_000,
		s3Key: "seed/food/fruit-bowl",
	},
	{
		title:       "Mechanical Keyboard Setup",
		description: "Minimal desk setup featuring a custom mechanical keyboard and dual monitors.",
		tags:        []string{"technology", "setup", "minimal"},
		width:       1920, height: 1080, fileSize: 1_700_000,
		s3Key: "seed/technology/keyboard-setup",
	},
	{
		title:       "Circuit Board Close-up",
		description: "Macro photography of a green PCB revealing intricate soldered components.",
		tags:        []string{"technology", "macro", "electronics"},
		width:       1600, height: 1067, fileSize: 2_050_000,
		s3Key: "seed/technology/circuit-board",
	},
	{
		title:       "Santorini Sunset",
		description: "Classic whitewashed buildings of Santorini bathed in warm sunset tones.",
		tags:        []string{"travel", "europe", "sunset"},
		width:       2400, height: 1600, fileSize: 3_300_000,
		s3Key: "seed/travel/santorini-sunset",
	},
	{
		title:       "Tokyo Neon Nights",
		description: "Busy neon-lit street in Shinjuku on a rainy evening.",
		tags:        []string{"travel", "asia", "urban"},
		width:       1080, height: 1620, fileSize: 2_100_000,
		s3Key: "seed/travel/tokyo-neon",
	},
}

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	// ── Seed users ────────────────────────────────────────────────────────────
	userIDs := map[string]string{} // email → uuid string

	for _, u := range defaultUsers {
		var id string
		err := pool.QueryRow(ctx, "SELECT id FROM users WHERE email=$1", u.email).Scan(&id)
		if err == nil {
			fmt.Printf("skip  %-30s (already exists)\n", u.email)
			userIDs[u.email] = id
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(u.password), 12)
		if err != nil {
			log.Fatalf("hash password for %s: %v", u.email, err)
		}

		err = pool.QueryRow(ctx,
			`INSERT INTO users (email, username, password_hash, role)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			u.email, u.username, string(hash), u.role,
		).Scan(&id)
		if err != nil {
			log.Fatalf("insert user %s: %v", u.email, err)
		}
		userIDs[u.email] = id
		fmt.Printf("seeded %-30s  role=%-5s  pass=%s\n", u.email, u.role, u.password)
	}

	// ── Seed media for testuser ───────────────────────────────────────────────
	testuserID, ok := userIDs["user@mediashare.local"]
	if !ok {
		log.Fatal("testuser ID not found; cannot seed media")
	}

	for _, m := range seedMediaItems {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM media WHERE original_key=$1)", m.s3Key+".jpg",
		).Scan(&exists)
		if err != nil {
			log.Fatalf("check media %q: %v", m.title, err)
		}
		if exists {
			fmt.Printf("skip  media %-40s (already exists)\n", m.title)
			continue
		}

		code := shortcode.Generate(8)

		var mediaID string
		err = pool.QueryRow(ctx,
			`INSERT INTO media
			   (user_id, short_code, type, title, description, tags,
			    status, original_key, file_size, mime_type, width, height)
			 VALUES ($1,$2,'image',$3,$4,$5,'ready',$6,$7,'image/jpeg',$8,$9)
			 RETURNING id`,
			testuserID, code, m.title, m.description, m.tags,
			m.s3Key+".jpg", m.fileSize, m.width, m.height,
		).Scan(&mediaID)
		if err != nil {
			log.Fatalf("insert media %q: %v", m.title, err)
		}

		// Insert thumbnail media_file variant
		_, err = pool.Exec(ctx,
			`INSERT INTO media_files (media_id, variant, s3_key, width, height, file_size, format)
			 VALUES ($1, 'thumbnail', $2, $3, $4, $5, 'jpeg')`,
			mediaID, m.s3Key+"_thumb.jpg", m.width/4, m.height/4, m.fileSize/10,
		)
		if err != nil {
			log.Fatalf("insert media_file for %q: %v", m.title, err)
		}

		fmt.Printf("seeded media %-40s  tags=%v  code=%s\n", m.title, m.tags, code)
	}

	// ── Seed reports ─────────────────────────────────────────────────────────
	adminID, ok := userIDs["admin@mediashare.local"]
	if !ok {
		log.Fatal("admin ID not found; cannot seed reports")
	}

	// Fetch the IDs of seeded media items by original_key
	type reportSeed struct {
		mediaKey   string // original_key of target media
		reason     string
		status     string // pending | resolved | dismissed
		reviewedAt *time.Time
	}

	now := time.Now()
	minus1h := now.Add(-1 * time.Hour)
	minus3h := now.Add(-3 * time.Hour)
	minus6h := now.Add(-6 * time.Hour)
	minus12h := now.Add(-12 * time.Hour)
	minus24h := now.Add(-24 * time.Hour)

	reportSeeds := []reportSeed{
		// pending — awaiting admin review
		{mediaKey: "seed/food/artisan-pizza.jpg", reason: "Possible copyright infringement — appears to be a stock photo used without license.", status: "pending"},
		{mediaKey: "seed/travel/tokyo-neon.jpg", reason: "Image contains personally identifiable individuals without visible consent.", status: "pending"},
		{mediaKey: "seed/portrait/market-day.jpg", reason: "Watermark from another platform visible in the corner.", status: "pending"},

		// resolved — admin reviewed and took action
		{mediaKey: "seed/technology/circuit-board.jpg", reason: "Content appears to violate export-control regulations for electronics schematics.", status: "resolved", reviewedAt: &minus1h},
		{mediaKey: "seed/animals/red-fox.jpg", reason: "Suspected AI-generated image uploaded as original photography.", status: "resolved", reviewedAt: &minus3h},
		{mediaKey: "seed/nature/forest-trail.jpg", reason: "Metadata stripped — suspected stolen photograph.", status: "resolved", reviewedAt: &minus6h},

		// dismissed — admin reviewed and found no violation
		{mediaKey: "seed/architecture/glass-tower.jpg", reason: "Building facade contains corporate logo — possible trademark issue.", status: "dismissed", reviewedAt: &minus12h},
		{mediaKey: "seed/portrait/golden-hour.jpg", reason: "Model appears underage.", status: "dismissed", reviewedAt: &minus24h},
		{mediaKey: "seed/food/fruit-bowl.jpg", reason: "Duplicate content already posted by another user.", status: "dismissed", reviewedAt: &minus24h},
	}

	for _, r := range reportSeeds {
		// Look up the media ID
		var mediaID string
		err := pool.QueryRow(ctx,
			"SELECT id FROM media WHERE original_key=$1", r.mediaKey,
		).Scan(&mediaID)
		if err != nil {
			fmt.Printf("skip  report for %q: media not found (run seed first)\n", r.mediaKey)
			continue
		}

		// Skip if a report from admin on this media already exists
		var exists bool
		_ = pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM reports WHERE media_id=$1 AND reporter_id=$2 AND reason=$3)",
			mediaID, adminID, r.reason,
		).Scan(&exists)
		if exists {
			fmt.Printf("skip  report %-45s (already exists)\n", r.mediaKey)
			continue
		}

		if r.status == "pending" {
			_, err = pool.Exec(ctx,
				`INSERT INTO reports (media_id, reporter_id, reason, status)
				 VALUES ($1, $2, $3, 'pending')`,
				mediaID, adminID, r.reason,
			)
		} else {
			_, err = pool.Exec(ctx,
				`INSERT INTO reports (media_id, reporter_id, reason, status, reviewed_by, reviewed_at)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				mediaID, adminID, r.reason, r.status, adminID, r.reviewedAt,
			)
		}
		if err != nil {
			log.Fatalf("insert report for %q: %v", r.mediaKey, err)
		}
		fmt.Printf("seeded report %-45s  status=%s\n", r.mediaKey, r.status)
	}

	fmt.Println("done.")
}
