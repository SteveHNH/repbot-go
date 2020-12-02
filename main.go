package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"

	"github.com/SteveHNH/repbot-go/config"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

const setRep = `UPDATE reputation SET rep = rep + 1, user = ? WHERE username = ?`
const getRep = `SELECT rep FROM reputation WHERE username = ?`
const initRep = `INSERT OR IGNORE into reputation (username, user, rep) values (?, ?, ?)`

func main() {
	cfg := config.Get()

	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("error creating Discord session: %s", err)
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	err = dg.Open()
	defer dg.Close()

	if err != nil {
		log.Fatalf("error opening Discord connection: %s", err)
	}

	log.Println("Bot is now running. Press CTRL+C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore repbot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Printf("received message: \"%s\" - @%s ", m.Content, m.Author.Username)

	var matched bool
	repAdd := `^\!rep\s\<\@\!*\d+\>\s*`
	repPing := `^\!rep\sping\s*`

	matched, _ = regexp.MatchString(repPing, m.Content)
	if matched {
		s.ChannelMessageSend(m.ChannelID, "pong")
	}

	matched, _ = regexp.MatchString(repAdd, m.Content)
	if matched {
		if m.Author.ID != m.Mentions[0].ID {
			repInc(m.Mentions[0], m, s)
		} else {
			s.ChannelMessageSend(m.ChannelID, "you can't update your own rep")
		}
	}
}

func checkUser(u *discordgo.User, db *sql.DB) (result bool, err error) {
	rows, err := db.Query(getRep, u.ID)
	if err != nil {
		return false, err
	}

	var repValue int
	for rows.Next() {
		rows.Scan(&repValue)
	}
	if repValue > 0 {
		return true, nil
	}
	return false, nil
}

func repInc(u *discordgo.User, m *discordgo.MessageCreate, s *discordgo.Session) {
	db, _ := sql.Open("sqlite3", config.Get().DB)
	defer db.Close()
	usercheck, err := checkUser(u, db)
	if err != nil {
		log.Printf("Something broke with userCheck: %s", err)
	}
	if usercheck == false {
		statement, err := db.Prepare(initRep)
		if err != nil {
			log.Printf("Unable to prepare sql statement for initRep: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not initialize new user "+u.Username)
		}
		_, err = statement.Exec(u.ID, u.Username, 1)
		if err != nil {
			log.Printf("initRep execution failed: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not initialize user "+u.Username)
		} else {
			s.ChannelMessageSend(m.ChannelID, "Intialized new user with 1 rep! Congrats, "+u.Username)
		}

	} else {
		statement, err := db.Prepare(setRep)
		if err != nil {
			log.Printf("Unable to prepare sql statement for repInc: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not update rep for "+u.Username)
		}
		_, err = statement.Exec(u.Username, u.ID)
		if err != nil {
			log.Printf("repInc execution failed: %s", err)
			s.ChannelMessageSend(m.ChannelID, "Could not update rep for "+u.Username)
		} else {
			rows, _ := db.Query(getRep, u.ID)
			var repValue int
			for rows.Next() {
				rows.Scan(&repValue)
			}
			s.ChannelMessageSend(m.ChannelID, "Rep increased to "+strconv.Itoa(repValue)+" for "+u.Username)
		}
	}
}
