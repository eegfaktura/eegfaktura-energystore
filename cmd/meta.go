package cmd

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"regexp"
	"sort"
)

type flagOptions struct {
	showTables               bool
	showHistogram            bool
	showKeys                 bool
	withPrefix               string
	keyLookup                string
	itemMeta                 bool
	keyHistory               bool
	showInternal             bool
	readOnly                 bool
	truncate                 bool
	encryptionKey            string
	checksumVerificationMode string
	discard                  bool
	externalMagicVersion     uint16
}

var (
	opt       flagOptions
	meterId   string
	metaAttr  string
	metaValue string
)

func init() {
	RootCmd.AddCommand(metaCmd)
	metaCmd.Flags().BoolVarP(&opt.showTables, "show-tables", "s", false,
		"If set to true, show tables as well.")
	metaCmd.Flags().BoolVar(&opt.showHistogram, "histogram", false,
		"Show a histogram of the key and value sizes.")
	metaCmd.Flags().BoolVar(&opt.showKeys, "show-keys", false, "Show keys stored in Badger")
	metaCmd.Flags().StringVar(&opt.withPrefix, "with-prefix", "",
		"Consider only the keys with specified prefix")
	metaCmd.Flags().StringVarP(&opt.keyLookup, "lookup", "l", "", "Hex of the key to lookup")
	metaCmd.Flags().BoolVar(&opt.itemMeta, "show-meta", true, "Output item meta data as well")
	metaCmd.Flags().BoolVar(&opt.keyHistory, "history", false, "Show all versions of a key")
	metaCmd.Flags().BoolVar(
		&opt.showInternal, "show-internal", false, "Show internal keys along with other keys."+
			" This option should be used along with --show-key option")
	metaCmd.Flags().BoolVar(&opt.readOnly, "read-only", true, "If set to true, DB will be opened "+
		"in read only mode. If DB has not been closed properly, this option can be set to false "+
		"to open DB.")
	metaCmd.Flags().BoolVar(&opt.truncate, "truncate", false, "If set to true, it allows "+
		"truncation of value log files if they have corrupt data.")
	metaCmd.Flags().StringVar(&opt.encryptionKey, "enc-key", "", "Use the provided encryption key")
	metaCmd.Flags().StringVar(&opt.checksumVerificationMode, "cv-mode", "none",
		"[none, table, block, tableAndBlock] Specifies when the db should verify checksum for SST.")
	metaCmd.Flags().BoolVar(&opt.discard, "discard", false,
		"Parse and print DISCARD file from value logs.")
	metaCmd.Flags().Uint16Var(&opt.externalMagicVersion, "external-magic", 0,
		"External magic number")

	metaCmd.AddCommand(metaSetCmd)
	metaSetCmd.Flags().StringVar(&meterId, "meter", "", "Meteringpoint to be modified")
	metaSetCmd.Flags().StringVar(&metaAttr, "attr", "", "Metadata Attribute")
	metaSetCmd.Flags().StringVar(&metaValue, "value", "", "Metadata Value")
}

var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Meta info of persist metering points",
	Long: `
This command prints information about the metring key-value store.  It reads MANIFEST and prints its
info.
`,
	RunE: handleMeta,
}

var metaSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set Meta info attribute of persisted metering points",
	Long: `
This command prints information about the metring key-value store.  It reads MANIFEST and prints its
info.
`,
	RunE: setMeta,
}

func handleMeta(cmd *cobra.Command, args []string) error {
	viper.Set("persistence.path", dir)
	db, err := store.OpenStorage(tenant)
	if err != nil {
		return err
	}
	defer db.Close()

	m, err := db.GetMeta("cpmeta/0")
	if err != nil {
		return err
	}

	cps := m.CounterPoints
	sort.Slice(cps, func(i, j int) bool {
		return cps[i].ID < cps[j].ID
	})

	fmt.Printf("%3s|%33s|%12s|%4s|%10s|%22s|%12s\n", "Id", "Name", "Direction", "Idx", "Cnt", "Begin", "End")
	for _, cp := range cps {
		fmt.Printf("%3s|%33s|%12s|%4d|%10d|%22s|%12s\n", cp.ID, cp.Name, cp.Dir, cp.SourceIdx, cp.Count, cp.PeriodStart, cp.PeriodEnd)
	}

	return nil
}

func setMeta(cmd *cobra.Command, args []string) error {
	viper.Set("persistence.path", dir)
	db, err := store.OpenStorage(tenant)
	if err != nil {
		return err
	}
	defer db.Close()

	m, err := db.GetMeta("cpmeta/0")
	if err != nil {
		return err
	}

	metaSet := map[string]*model.CounterPointMeta{}
	for _, meta := range m.CounterPoints {
		metaSet[meta.Name] = meta
	}
	metaMeterId := metaSet[meterId]
	switch metaAttr {
	case "begin":
		if err := checkDateValue(metaValue); err != nil {
			return err
		}
		metaMeterId.PeriodStart = metaValue
	case "end":
		if err := checkDateValue(metaValue); err != nil {
			return err
		}
		metaMeterId.PeriodEnd = metaValue
	default:
		return errors.New(fmt.Sprintf("Unsupported metadata attribute %s", metaAttr))
	}

	return db.SetMeta(m)
}

func checkDateValue(date string) error {
	dateLine := regexp.MustCompile(`^[0-9]{2}.[0-9]{2}.[0-9]{4}\s[0-9]{2}:[0-9]{2}:[0-9]{2}$`)
	if dateLine.MatchString(date) {
		return nil
	}
	return errors.New("Wrong Date Format: Expected 'MM.DD.YYYY HH:MM:SS'")
}
