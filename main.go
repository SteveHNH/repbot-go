package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/SteveHNH/repbot-go/config"
	"github.com/bwmarrin/discordgo"
	"github.com/jedib0t/go-pretty/table"
	_ "github.com/mattn/go-sqlite3"
)

const dbDriver string = "sqlite3"

const setRep = `UPDATE reputation SET rep = rep + 1, user = ? WHERE username = ?`
const getRep = `SELECT rep FROM reputation WHERE username = ?`
const createUser = `INSERT OR IGNORE into reputation (username, user, rep) values (?, ?, ?)`
const getRank = `SELECT user, rep from reputation ORDER BY rep DESC, user ASC LIMIT 10`
const getTbl = `SELECT name FROM sqlite_master WHERE type='table' AND name='reputation';`
const createTbl = `CREATE TABLE reputation (username TEXT PRIMARY KEY, rep INTEGER DEFAULT 0, user VARCHAR);`
const getUsers = `SELECT username, user FROM 'reputation';`
const updateUserName = `UPDATE reputation SET user = ? WHERE username = ?;`
const getUserRank = `SELECT COUNT(*) + 1 FROM reputation WHERE rep > ?`

var configFile string

var (
	reRepAdd  = regexp.MustCompile(`^\!rep\s\<\@\!*\d+\>\s*`)
	reRepRank = regexp.MustCompile(`^\!rep\srank\s*`)
	reRepPing = regexp.MustCompile(`^\!rep\sping\s*`)
	reRepMe   = regexp.MustCompile(`^\!rep\sme\s*`)
)

var milestones = []int{10, 25, 50, 100, 200, 300, 400, 500}

type repbotClient struct {
	cfg        *config.Config
	ds         *discordgo.Session
	db         *sql.DB
	token      string
	datasource string
}

var client *repbotClient

// init zero parses command line flags
func init() {
	// No default value
	flag.StringVar(&configFile, "c", "", "Specify a path to the config file")
	flag.Parse()
}

func (c *repbotClient) init() (*repbotClient, error) {
	var err error

	cfg := config.Get(configFile)

	c = &repbotClient{
		cfg:        cfg,
		token:      cfg.Token,
		datasource: cfg.DB,
	}

	c.ds, err = discordgo.New("Bot " + c.token)
	if err != nil {
		return c, fmt.Errorf("error creating Discord session: %s", err)
	}

	return c, nil
}

// init the second checks to see if the database needs to be setup before the main function runs
func checkDatabase() {
	tableExists, err := checkTable(client.db)
	if err != nil {
		log.Fatalf("failed checking table 'reputation' exists: %s", err)
	}

	if !tableExists {
		log.Printf("no table 'reputation' found: creating")
		err := createTable(client.db)
		if err != nil {
			log.Fatalf("failed creating table 'reputation': %s", err)
		}
	}

	tableExists, err = checkTable(client.db)
	if err != nil {
		log.Fatalf("failed checking table 'reputation' exists: %s", err)
	}

	if !tableExists {
		log.Fatalf("database setup failed: table creation failed")
	}
}

func updateUsers() {
	rows, err := client.db.Query(getUsers)
	if err != nil {
		log.Printf("error querying users: %s", err)
		return
	}
	defer rows.Close()

	var userID string
	var user string
	people := make(map[string]string)

	for rows.Next() {
		if err := rows.Scan(&userID, &user); err != nil {
			log.Printf("error scanning user row: %s", err)
			continue
		}
		u, err := client.ds.User(userID)
		if err != nil {
			log.Printf("Error retrieving user details: %s\n", err)
			continue
		}

		if u.Username != user {
			log.Printf("Username Mismatch - User: %s, Discord: %s, Repbot: %s. Queuing for update.\n", userID, u.Username, user)

			// Careful - u.Username is the nick according to Discord
			// and userID is the ID according to Repbot's db
			people[userID] = u.Username
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating user rows: %s", err)
	}

	if len(people) == 0 {
		log.Println("No out-of-date users found")
		return
	}

	log.Println("Updating out-of-date users")
	for id, username := range people {
		statement, err := client.db.Prepare(updateUserName)
		if err != nil {
			log.Printf("Unable to prepare sql statement for user update: %s", err)
			continue
		}

		// Careful - u.Username is the nick according to Discord
		// and userID is the ID according to Repbot's db
		// "Update reputation set user = username where username = id"
		_, err = statement.Exec(username, id)
		if err != nil {
			log.Printf("user update execution failed: %s", err)
		}
		statement.Close()
	}
}

func main() {
	var err error

	client = &repbotClient{}
	client, err = client.init()
	if err != nil {
		log.Fatalln(err)
	}

	client.db, err = sql.Open(dbDriver, client.datasource)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.db.Close()

	// Check if the databse is setup
	checkDatabase()

	client.ds.AddHandler(messageCreate)
	client.ds.AddHandler(messageReactionAdd)
	client.ds.Identify.Intents = discordgo.MakeIntent(
		discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions,
	)

	err = client.ds.Open()
	if err != nil {
		log.Fatalf("error opening Discord connection: %s", err)
	}
	defer client.ds.Close()

	log.Println("checking for user updates from Discord")
	updateUsers()
	log.Println("completed checking users for updates")

	log.Println("Bot is now running. Press CTRL+C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore repbot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Printf("received message: \"%s\" - @%s - %s", m.Content, m.Author.Username, m.Author.ID)

	if reRepPing.MatchString(m.Content) {
		log.Printf("Ping request received")
		s.ChannelMessageSend(m.ChannelID, "pong")
		return
	}

	if reRepRank.MatchString(m.Content) {
		if len(m.Mentions) > 0 {
			log.Printf("Rank request for user received")
			repRankUser(m, s)
		} else {
			log.Printf("Rank request received")
			repRankAll(m, s)
		}
		return
	}

	if reRepMe.MatchString(m.Content) {
		log.Printf("Rep me request received")
		repMe(m, s)
		return
	}

	if reRepAdd.MatchString(m.Content) {
		log.Printf("Rep increase request received")
		if m.Author.ID != m.Mentions[0].ID {
			repInc(m.Mentions[0], m.ChannelID, s)
			return
		}

		log.Printf("Ignoring greedy rep request")
		s.ChannelMessageSend(m.ChannelID, "You can't update your own rep")
	}
}

// checkUser returns true if the user exists in the db, or false if not
func checkUser(u *discordgo.User, db *sql.DB) (result bool, err error) {
	rows, err := client.db.Query(getRep, u.ID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	exists := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

// repRankAll queries the database for all users, orders them by rep and sends a table with this info to the channel
func repRankAll(m *discordgo.MessageCreate, s *discordgo.Session) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Rep", "Title", "User"})

	rows, err := client.db.Query(getRank)
	if err != nil {
		log.Printf("error querying rank: %s", err)
		s.ChannelMessageSend(m.ChannelID, "Could not retrieve rankings")
		return
	}
	defer rows.Close()

	var userName string
	var repValue int
	pos := 0

	for rows.Next() {
		if err := rows.Scan(&userName, &repValue); err != nil {
			log.Printf("error scanning rank row: %s", err)
			continue
		}
		pos++
		t.AppendRow([]interface{}{pos, repValue, rankTitle(repValue), userName})
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating rank rows: %s", err)
	}

	t.SetStyle(table.StyleLight)
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```\n%s\n```", t.Render()))
}

// repRankUser reports the rep and leaderboard rank for a specific mentioned user.
func repRankUser(m *discordgo.MessageCreate, s *discordgo.Session) {
	u := m.Mentions[0]

	var rep int
	if err := client.db.QueryRow(getRep, u.ID).Scan(&rep); err != nil {
		if err == sql.ErrNoRows {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s has no rep yet.", u.Username))
		} else {
			log.Printf("error querying rep for repRankUser: %s", err)
		}
		return
	}

	var rank int
	if err := client.db.QueryRow(getUserRank, rep).Scan(&rank); err != nil {
		log.Printf("error querying rank for repRankUser: %s", err)
		return
	}

	s.ChannelMessageSend(m.ChannelID,
		fmt.Sprintf("%s's rep is %d | Rank: #%d", u.Username, rep, rank))
}

func repInc(u *discordgo.User, channelID string, s *discordgo.Session) {

	// Check if user exists
	usercheck, err := checkUser(u, client.db)
	if err != nil {
		log.Printf("Something broke with userCheck: %s", err)
		return
	}

	// User doesn't exist yet — create them
	if usercheck == false {
		statement, err := client.db.Prepare(createUser)
		if err != nil {
			log.Printf("Unable to prepare sql statement for initRep: %s", err)
			s.ChannelMessageSend(channelID, "Could not initialize new user "+u.Username)
			return
		}
		defer statement.Close()

		_, err = statement.Exec(u.ID, u.Username, 0)
		if err != nil {
			log.Printf("initRep execution failed: %s", err)
			s.ChannelMessageSend(channelID, "Could not initialize user "+u.Username)
			return
		}
		log.Printf("created new user: %s", u.Username)
		s.ChannelMessageSend(channelID, fmt.Sprintf("Welcome new user %s!", u.Username))
	}

	// Increment rep
	statement, err := client.db.Prepare(setRep)
	if err != nil {
		log.Printf("Unable to prepare sql statement for repInc: %s", err)
		s.ChannelMessageSend(channelID, "Could not update rep for "+u.Username)
		return
	}
	defer statement.Close()

	_, err = statement.Exec(u.Username, u.ID)
	if err != nil {
		log.Printf("repInc execution failed: %s", err)
		s.ChannelMessageSend(channelID, "Could not update rep for "+u.Username)
		return
	}

	rows, err := client.db.Query(getRep, u.ID)
	if err != nil {
		log.Printf("error querying rep for %s: %s", u.Username, err)
		s.ChannelMessageSend(channelID, fmt.Sprintf("Rep updated for %s", u.Username))
		return
	}
	defer rows.Close()

	var repValue int
	if rows.Next() {
		if err := rows.Scan(&repValue); err != nil {
			log.Printf("error scanning rep value: %s", err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating rep rows: %s", err)
	}

	if isMilestone(repValue) {
		s.ChannelMessageSend(channelID, fmt.Sprintf("**%s just hit %d rep!**", u.Username, repValue))
	} else {
		s.ChannelMessageSend(channelID, fmt.Sprintf("Rep increased to %d for %s", repValue, u.Username))
	}
}

// checkTable checks to see if a table named "reputation" exists
func checkTable(db *sql.DB) (bool, error) {
	rows, err := db.Query(getTbl)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var t string
	if rows.Next() {
		if err := rows.Scan(&t); err != nil {
			return false, err
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	return t == "reputation", nil
}

// createTable creates the "reputation" table with the schema described in the initDB constant
func createTable(d *sql.DB) error {
	_, err := d.Exec(createTbl)
	if err != nil {
		return err
	}

	return nil
}

// messageReactionAdd fires when any reaction is added to a message.
// Reacting with LODLove01 gives rep to the message author.
func messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}
	if r.Emoji.Name != "LODLove01" {
		return
	}
	msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		log.Printf("error fetching message for reaction rep: %s", err)
		return
	}
	if msg.Author == nil || msg.Author.Bot || msg.Author.ID == r.UserID {
		return
	}
	repInc(msg.Author, r.ChannelID, s)
}

// repMe reports the calling user's current rep and leaderboard rank.
func repMe(m *discordgo.MessageCreate, s *discordgo.Session) {
	rows, err := client.db.Query(getRep, m.Author.ID)
	if err != nil {
		log.Printf("error querying rep for repMe: %s", err)
		return
	}
	defer rows.Close()

	if !rows.Next() {
		s.ChannelMessageSend(m.ChannelID, "You have no rep yet. Go be helpful!")
		return
	}
	var rep int
	if err := rows.Scan(&rep); err != nil {
		log.Printf("error scanning rep for repMe: %s", err)
		return
	}
	rows.Close()

	var rank int
	if err := client.db.QueryRow(getUserRank, rep).Scan(&rank); err != nil {
		log.Printf("error querying rank for repMe: %s", err)
		return
	}

	s.ChannelMessageSend(m.ChannelID,
		fmt.Sprintf("%s — Rep: %d | Rank: #%d", m.Author.Username, rep, rank))
}

// isMilestone returns true if the given rep value is a notable milestone.
func isMilestone(rep int) bool {
	for _, m := range milestones {
		if rep == m {
			return true
		}
	}
	return false
}

// rankTitle returns the display title for a given rep value.
func rankTitle(rep int) string {
	switch {
	case rep >= 500:
		return "Icon"
	case rep >= 300:
		return "Legend"
	case rep >= 200:
		return "Champion"
	case rep >= 100:
		return "Elite"
	case rep >= 50:
		return "Veteran"
	case rep >= 25:
		return "Member"
	case rep >= 10:
		return "Regular"
	default:
		return "Newcomer"
	}
}
