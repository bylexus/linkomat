package api

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gorm.io/gorm"
)

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users (operates on the meta DB only)",
	}
	cmd.AddCommand(newUserCreateCmd())
	cmd.AddCommand(newUserSetPasswordCmd())
	cmd.AddCommand(newUserActivateCmd())
	cmd.AddCommand(newUserDeactivateCmd())
	cmd.AddCommand(newUserDeleteCmd())
	cmd.AddCommand(newUserListCmd())
	cmd.AddCommand(newUserPurgeCmd())
	cmd.AddCommand(newUserFriendsCmd())
	cmd.AddCommand(newUserRemoveFriendshipCmd())
	return cmd
}

func newUserCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <username>",
		Short: "Create a new user and prompt for a password",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()

			username := args[0]
			var existing User
			err = meta.Where("username = ?", username).First(&existing).Error
			if err == nil {
				return fmt.Errorf("user %q already exists", username)
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			pw, err := promptPassword("Password: ")
			if err != nil {
				return err
			}
			if len(pw) == 0 {
				return errors.New("password must not be empty")
			}
			pw2, err := promptPassword("Confirm: ")
			if err != nil {
				return err
			}
			if pw != pw2 {
				return errors.New("passwords do not match")
			}
			hash, err := hashPassword(pw)
			if err != nil {
				return err
			}
			u := User{
				Username:     username,
				PasswordHash: hash,
				Active:       true,
				CreatedAt:    time.Now(),
			}
			if err := meta.Create(&u).Error; err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created user %q (id=%d)\n", u.Username, u.ID)
			return nil
		},
	}
}

func newUserSetPasswordCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-password <username>",
		Short: "Replace the user's password and revoke all tokens",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()

			username := args[0]
			var u User
			if err := meta.Where("username = ?", username).First(&u).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("user %q does not exist", username)
				}
				return err
			}
			pw, err := promptPassword("New password: ")
			if err != nil {
				return err
			}
			if len(pw) == 0 {
				return errors.New("password must not be empty")
			}
			pw2, err := promptPassword("Confirm: ")
			if err != nil {
				return err
			}
			if pw != pw2 {
				return errors.New("passwords do not match")
			}
			hash, err := hashPassword(pw)
			if err != nil {
				return err
			}
			if err := meta.Model(&u).Update("password_hash", hash).Error; err != nil {
				return err
			}
			res := meta.Model(&Token{}).Where("user_id = ? AND revoked_at IS NULL", u.ID).
				Update("revoked_at", time.Now())
			if res.Error != nil {
				return res.Error
			}
			fmt.Fprintf(cmd.OutOrStdout(), "password updated for %q; %d tokens revoked\n", u.Username, res.RowsAffected)
			return nil
		},
	}
}

func newUserActivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activate <username>",
		Short: "Mark the user as active",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setUserActive(cmd, args[0], true)
		},
	}
}

func newUserDeactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate <username>",
		Short: "Mark the user as inactive and revoke all tokens",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()
			var u User
			if err := meta.Where("username = ?", args[0]).First(&u).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("user %q does not exist", args[0])
				}
				return err
			}
			if err := meta.Model(&u).Update("active", false).Error; err != nil {
				return err
			}
			if err := meta.RevokeAllUserTokens(u.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deactivated %q and revoked tokens\n", u.Username)
			return nil
		},
	}
}

func setUserActive(cmd *cobra.Command, username string, active bool) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	meta, err := OpenMetaDB(cfg)
	if err != nil {
		return err
	}
	defer meta.Close()
	var u User
	if err := meta.Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("user %q does not exist", username)
		}
		return err
	}
	if err := meta.Model(&u).Update("active", active).Error; err != nil {
		return err
	}
	if active {
		fmt.Fprintf(cmd.OutOrStdout(), "activated %q\n", u.Username)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "deactivated %q\n", u.Username)
	}
	return nil
}

func newUserDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <username>",
		Short: "Hard-delete user (also removes the per-user DB file in v1)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()
			username := args[0]
			var u User
			if err := meta.Where("username = ?", username).First(&u).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("user %q does not exist", username)
				}
				return err
			}
			if err := meta.RevokeAllUserTokens(u.ID); err != nil {
				return err
			}
			if err := meta.Delete(&u).Error; err != nil {
				return err
			}
			base := filepath.Join(cfg.DataDir, "users", username+".db")
			for _, p := range []string{base, base + "-shm", base + "-wal"} {
				if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove %s: %v\n", p, err)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted user %q\n", username)
			return nil
		},
	}
}

func newUserListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()
			var users []User
			if err := meta.Order("username asc").Find(&users).Error; err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tUSERNAME\tACTIVE\tCREATED")
			for _, u := range users {
				fmt.Fprintf(tw, "%d\t%s\t%t\t%s\n", u.ID, u.Username, u.Active, u.CreatedAt.Format(time.RFC3339))
			}
			return tw.Flush()
		},
	}
}

func newUserPurgeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "purge",
		Short: "Manually clean up expired tokens from the meta DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()
			res := meta.Where("expires_at < ?", time.Now()).Delete(&Token{})
			if res.Error != nil {
				return res.Error
			}
			fmt.Fprintf(cmd.OutOrStdout(), "purged %d expired token rows\n", res.RowsAffected)
			return nil
		},
	}
}

func newUserFriendsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "friends <username>",
		Short: "List all friendships of the user (all states)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()
			u, err := findUserByName(meta, args[0])
			if err != nil {
				return err
			}
			var rows []Friendship
			err = meta.Preload("RequestedBy").Preload("AcceptedBy").
				Where("requested_by_id = ? OR accepted_by_id = ?", u.ID, u.ID).
				Order("requested_at desc").
				Find(&rows).Error
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tREQUESTED_BY\tACCEPTED_BY\tSTATUS\tREQUESTED_AT\tACCEPTED_AT\tDECLINED_AT")
			for i := range rows {
				f := &rows[i]
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					f.ID, f.RequestedBy.Username, f.AcceptedBy.Username, friendshipStatus(f),
					f.RequestedAt.Format(time.RFC3339), formatOptionalTime(f.AcceptedAt), formatOptionalTime(f.DeclinedAt))
			}
			return tw.Flush()
		},
	}
}

func newUserRemoveFriendshipCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove-friendship <id> | <user1> <user2>",
		Short: "Delete a friendship by ID, or the one between two users (either direction)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()
			q := meta.Preload("RequestedBy").Preload("AcceptedBy")
			var notFound error
			if len(args) == 1 {
				q = q.Where("id = ?", args[0])
				notFound = fmt.Errorf("friendship %q not found", args[0])
			} else {
				u1, err := findUserByName(meta, args[0])
				if err != nil {
					return err
				}
				u2, err := findUserByName(meta, args[1])
				if err != nil {
					return err
				}
				q = q.Where("(requested_by_id = ? AND accepted_by_id = ?) OR (requested_by_id = ? AND accepted_by_id = ?)",
					u1.ID, u2.ID, u2.ID, u1.ID)
				notFound = fmt.Errorf("no friendship between %q and %q", args[0], args[1])
			}
			var f Friendship
			if err := q.First(&f).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return notFound
				}
				return err
			}
			if err := meta.Where("id = ?", f.ID).Delete(&Friendship{}).Error; err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed friendship %s (%s -> %s)\n", f.ID, f.RequestedBy.Username, f.AcceptedBy.Username)
			return nil
		},
	}
}

func findUserByName(meta *MetaDB, username string) (User, error) {
	var u User
	if err := meta.Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return u, fmt.Errorf("user %q does not exist", username)
		}
		return u, err
	}
	return u, nil
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format(time.RFC3339)
}

var stdinReader = bufio.NewReader(os.Stdin)

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := stdinReader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	b, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(b), nil
}