// Copyright (C) 2019-2026 Algorand Foundation Ltd.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package main

//go:generate ./bundle_genesis_json.sh

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/algorand/go-algorand/cmd/util/datadir"
	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated/model"
	"github.com/algorand/go-algorand/ledger/ledgercore"
	"github.com/algorand/go-algorand/libgoal"
	naddr "github.com/algorand/go-algorand/network/addr"
	"github.com/algorand/go-algorand/nodecontrol"
	"github.com/algorand/go-algorand/util"
	"github.com/algorand/go-algorand/util/tokens"
)

var peerDial string
var listenIP string
var targetDir string
var noLedger bool
var runUnderHost bool
var peersVerbose bool
var telemetryOverride string
var maxPendingTransactions uint64
var waitSec uint32
var newNodeNetwork string
var newNodeDestination string
var newNodeArchival bool
var newNodeRelay string
var newNodeFullConfig bool
var watchMillisecond uint64
var abortCatchup bool
var fastCatchupForce bool
var minCatchupRounds uint64

const catchpointURL = "https://algorand-catchpoints.s3.us-east-2.amazonaws.com/channel/%s/latest.catchpoint"

func init() {
	nodeCmd.AddCommand(startCmd)
	nodeCmd.AddCommand(stopCmd)
	nodeCmd.AddCommand(statusCmd)
	nodeCmd.AddCommand(lastroundCmd)
	nodeCmd.AddCommand(restartCmd)
	nodeCmd.AddCommand(cloneCmd)
	nodeCmd.AddCommand(generateTokenCmd)
	nodeCmd.AddCommand(pendingTxnsCmd)
	nodeCmd.AddCommand(waitCmd)
	nodeCmd.AddCommand(createCmd)
	nodeCmd.AddCommand(catchupCmd)
	nodeCmd.AddCommand(peersCmd)
	// Once the server-side implementation of the shutdown command is ready, we should enable this one.
	//nodeCmd.AddCommand(shutdownCmd)
	nodeCmd.AddCommand(p2pID)

	peersCmd.Flags().BoolVarP(&peersVerbose, "verbose", "v", false, "Also show each peer's ID and supported protocols")

	startCmd.Flags().StringVarP(&peerDial, "peer", "p", "", "Peer address to dial for initial connection")
	startCmd.Flags().StringVarP(&listenIP, "listen", "l", "", "Endpoint / REST address to listen on")
	startCmd.Flags().BoolVarP(&runUnderHost, "hosted", "H", false, "Run algod hosted by algoh")
	startCmd.Flags().StringVarP(&telemetryOverride, "telemetry", "t", "", `Enable telemetry if supported (Use "true", "false", "0" or "1")`)

	restartCmd.Flags().StringVarP(&peerDial, "peer", "p", "", "Peer address to dial for initial connection")
	restartCmd.Flags().StringVarP(&listenIP, "listen", "l", "", "Endpoint / REST address to listen on")
	restartCmd.Flags().BoolVarP(&runUnderHost, "hosted", "H", false, "Run algod hosted by algoh")
	restartCmd.Flags().StringVarP(&telemetryOverride, "telemetry", "t", "", `Enable telemetry if supported (Use "true", "false", "0" or "1")`)

	cloneCmd.Flags().StringVarP(&targetDir, "targetdir", "t", "", "Target directory for the clone")
	cloneCmd.Flags().BoolVarP(&noLedger, "noledger", "n", false, "Don't include ledger when copying (No Ledger)")

	localDefaults := config.GetDefaultLocal()
	createCmd.Flags().StringVar(&newNodeNetwork, "network", "", "Network the new node should point to")
	createCmd.Flags().StringVar(&newNodeDestination, "destination", "", "Destination path for the new node")
	createCmd.Flags().BoolVarP(&newNodeArchival, "archival", "a", localDefaults.Archival, "Make the new node archival, storing all blocks")
	createCmd.Flags().BoolVarP(&runUnderHost, "hosted", "H", localDefaults.RunHosted, "Configure the new node to run hosted by algoh")

	createCmd.Flags().StringVar(&newNodeRelay, "relay", localDefaults.NetAddress, "Configure as a relay with specified listening address (NetAddress)")
	createCmd.Flags().StringVar(&listenIP, "api", "", "REST API Endpoint")
	createCmd.Flags().BoolVar(&newNodeFullConfig, "full-config", false, "Store full config file")
	createCmd.MarkFlagRequired("destination")
	createCmd.MarkFlagRequired("network")

	pendingTxnsCmd.Flags().Uint64VarP(&maxPendingTransactions, "maxPendingTxn", "m", 0, "Cap the number of txns to fetch")
	waitCmd.Flags().Uint32VarP(&waitSec, "waittime", "w", 5, "Time (in seconds) to wait for node to make progress")
	statusCmd.Flags().Uint64VarP(&watchMillisecond, "watch", "w", 0, "Time (in milliseconds) between two successive status updates")

	catchupCmd.Flags().BoolVarP(&abortCatchup, "abort", "x", false, "Aborts the current catchup process")
	catchupCmd.Flags().BoolVar(&fastCatchupForce, "force", false, "Forces fast catchup with implicit catchpoint to start without a consent prompt")
	catchupCmd.Flags().Uint64VarP(&minCatchupRounds, "min", "m", 0, "Catchup only if the catchpoint would advance the node by the specified minimum number of rounds")

}

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage a specified algorand node",
	Long:  `Collection of commands to support the creation and management of Algorand node instances, where each instance corresponds to a unique data directory.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		//Fall back
		cmd.HelpFunc()(cmd, args)
	},
}

func getMissingCatchpointLabel(URL string) (label string, err error) {
	resp, err := http.Get(URL)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = errors.New(resp.Status)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	label = string(body)
	label = strings.TrimSuffix(label, "\n")

	// check if label is a valid catchpoint label
	_, _, err = ledgercore.ParseCatchpointLabel(label)
	if err != nil {
		return
	}
	return
}

var catchupCmd = &cobra.Command{
	Use:     "catchup",
	Short:   "Catchup the Algorand node to a specific catchpoint",
	Long:    "Catchup allows making large jumps over round ranges without the need to incrementally validate each individual round. Using external catchpoints is not a secure practice and should not be done for consensus participating nodes.\nIf no catchpoint is provided, this command attempts to lookup the latest catchpoint from algorand-catchpoints.s3.us-east-2.amazonaws.com.",
	Example: "goal node catchup 6500000#1234567890ABCDEF01234567890ABCDEF0\tStart catching up to round 6500000 with the provided catchpoint\ngoal node catchup --abort\t\t\t\t\tAbort the current catchup",
	Args:    catchpointCmdArgument,
	Run: func(cmd *cobra.Command, args []string) {
		var catchpoint string
		// assume first positional parameter is the catchpoint
		if len(args) != 0 {
			catchpoint = args[0]
		}
		datadir.OnDataDirs(func(dataDir string) {
			client := ensureAlgodClient(dataDir)

			if abortCatchup {
				err := client.AbortCatchup()
				if err != nil {
					reportErrorf(errorNodeStatus, err)
				}
				return
			}

			// lookup missing catchpoint
			if catchpoint == "" {
				vers, err := client.AlgodVersions()
				if err != nil {
					reportErrorf(errorNodeStatus, err)
				}
				genesis := strings.Split(vers.GenesisID, "-")[0]
				URL := fmt.Sprintf(catchpointURL, genesis)
				catchpoint, err = getMissingCatchpointLabel(URL)
				if err != nil {
					reportErrorf(errorCatchpointLabelMissing, errorUnableToLookupCatchpointLabel, err.Error())
				}

				// Prompt user to confirm using an implicit catchpoint.
				if !fastCatchupForce {
					fmt.Printf(nodeConfirmImplicitCatchpoint, catchpoint)
					reader := bufio.NewReader(os.Stdin)
					text, _ := reader.ReadString('\n')
					text = strings.ReplaceAll(text, "\n", "")
					if text != "yes" {
						reportErrorf(errorAbortedPerUserRequest)
					}
				}
			}

			resp, err := client.Catchup(catchpoint, minCatchupRounds)
			if err != nil {
				reportErrorf(errorNodeStatus, err)
			}
			if resp.CatchupMessage != catchpoint {
				reportInfof("node response: %s", resp.CatchupMessage)
			}
		})
	},
}

func catchpointCmdArgument(cmd *cobra.Command, args []string) error {
	catchpointsCount := 0
	for _, arg := range args {
		_, _, err := ledgercore.ParseCatchpointLabel(arg)
		switch err {
		case nil:
			if catchpointsCount > 0 {
				return errors.New(errorTooManyCatchpointLabels)
			}
			catchpointsCount++
			continue
		case ledgercore.ErrCatchpointParsingFailed:
			// this isn't a valid catchpoint label.
			// return a nice formatted error
			return errors.New(errorCatchpointLabelParsingFailed)
		default:
			return err
		}
	}
	return nil
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Initialize the specified Algorand node",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		if !verifyPeerDialArg() {
			return
		}
		binDir, err := util.ExeDir()
		if err != nil {
			panic(err)
		}
		datadir.OnDataDirs(func(dataDir string) {
			if libgoal.AlgorandDaemonSystemdManaged(dataDir) {
				reportErrorf(errorNodeManagedBySystemd)
			}

			nc := nodecontrol.MakeNodeController(binDir, dataDir)
			nodeArgs := nodecontrol.AlgodStartArgs{
				PeerAddress:       peerDial,
				ListenIP:          listenIP,
				RedirectOutput:    false,
				RunUnderHost:      runUnderHost,
				TelemetryOverride: telemetryOverride,
			}

			if getRunHostedConfigFlag(dataDir) {
				nodeArgs.RunUnderHost = true
			}

			algodAlreadyRunning, err := nc.StartAlgod(nodeArgs)
			if err != nil {
				reportErrorf(errorNodeFailedToStart, err)
			} else {
				if algodAlreadyRunning {
					reportInfoln(infoNodeAlreadyStarted)
				} else {
					reportInfoln(infoNodeStart)
				}
			}
		})
	},
}

func getRunHostedConfigFlag(dataDir string) bool {
	// See if this instance wants to run Hosted, even if '-H' wasn't specified on our cmdline
	cfg, err := config.LoadConfigFromDisk(dataDir)
	if err != nil && !os.IsNotExist(err) {
		reportErrorf(errLoadingConfig, dataDir, err)
	}
	return cfg.RunHosted
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the specified Algorand node",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		binDir, err := util.ExeDir()
		if err != nil {
			panic(err)
		}
		datadir.OnDataDirs(func(dataDir string) {
			if libgoal.AlgorandDaemonSystemdManaged(dataDir) {
				reportErrorf(errorNodeManagedBySystemd)
			}

			nc := nodecontrol.MakeNodeController(binDir, dataDir)

			log.Info(infoTryingToStopNode)

			err = nc.FullStop()
			if err != nil {
				reportErrorf(errorKill, err)
			}

			reportInfoln(infoNodeSuccessfullyStopped)
		})
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Stop, and then start, the specified Algorand node",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		if !verifyPeerDialArg() {
			return
		}
		binDir, err := util.ExeDir()
		if err != nil {
			panic(err)
		}
		datadir.OnDataDirs(func(dataDir string) {
			if libgoal.AlgorandDaemonSystemdManaged(dataDir) {
				reportErrorf(errorNodeManagedBySystemd)
			}

			nc := nodecontrol.MakeNodeController(binDir, dataDir)

			_, err = nc.GetAlgodPID()

			if err != nil {
				reportInfof(errorNodeNotDetected, err)
				fmt.Println("Attempting to start the Algorand node anyway...")
			} else {
				log.Info(infoTryingToStopNode)
				err = nc.FullStop()
				if err != nil {
					reportInfof(errorKill, err)
					fmt.Println("Attempting to start the Algorand node anyway...")
				} else {
					reportInfoln(infoNodeSuccessfullyStopped)
				}
			}
			// brief sleep to allow the node to finish shutting down
			time.Sleep(time.Duration(time.Second))

			nodeArgs := nodecontrol.AlgodStartArgs{
				PeerAddress:       peerDial,
				ListenIP:          listenIP,
				RedirectOutput:    false,
				RunUnderHost:      runUnderHost,
				TelemetryOverride: telemetryOverride,
			}

			if getRunHostedConfigFlag(dataDir) {
				nodeArgs.RunUnderHost = true
			}

			algodAlreadyRunning, err := nc.StartAlgod(nodeArgs)
			if err != nil {
				reportErrorf(errorNodeFailedToStart, err)
			} else {
				if algodAlreadyRunning {
					// This can never happen. In case it does, report about it.
					reportInfoln(infoNodeDidNotRestart)
				} else {
					reportInfoln(infoNodeStart)
				}
			}
		})
	},
}

var generateTokenCmd = &cobra.Command{
	Use:   "generatetoken",
	Short: "Generate and install a new API token",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		datadir.OnDataDirs(func(dataDir string) {
			// Ensure the node is stopped -- HealthCheck should fail
			clientConfig := libgoal.ClientConfig{
				AlgodDataDir: dataDir,
				KMDDataDir:   resolveKmdDataDir(dataDir),
				CacheDir:     ensureCacheDir(dataDir),
			}
			client, err := libgoal.MakeClientFromConfig(clientConfig, libgoal.AlgodClient)
			if err == nil {
				err = client.HealthCheck()
				if err == nil {
					reportErrorln(errorNodeRunning)
				}
			}

			// Generate & persist a new token
			apiToken, err := tokens.GenerateAPIToken(dataDir, tokens.AlgodTokenFilename)
			if err != nil {
				reportErrorf(errorNodeFailGenToken, err)
			}

			// Report the new token back to the user
			reportInfof(infoNodeWroteToken, apiToken)
		})
	},
}

var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "Get peers",
	Long: `Show the connections this node holds, the streams active on each, and traffic totals.

STREAMS lists the protocols with a stream open at the moment of the query, so it can change between calls. On a p2p network a connection may carry the gossip stream, pubsub (which also propagates transactions), DHT, or only libp2p machinery such as identify. Pass --verbose to also show each peer's ID and the protocols it supports, as learned from libp2p identify.

RECEIVED and SENT are cumulative byte counts. For a ws peer they cover that connection. For a p2p peer they cover all of this node's connections to that peer ID, across every libp2p protocol, and are shown on the peer's first row only. They are approximate: traffic exchanged before a connection is registered is missed, and a rapid reconnect can carry over the previous totals.`,
	Args: validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		datadir.OnDataDirs(getPeers)
	},
}

func getPeers(dataDir string) {

	client := ensureAlgodClient(dataDir)
	response, err := client.GetPeers()
	if err != nil {
		reportErrorf(errorNodePeers, err)
	}

	printPeers(os.Stdout, response.Peers, peersVerbose)
}

// printPeers writes the peer table. Rows keep the order algod returned them
// in. A trailing PEER column, holding the tail of the libp2p peer ID, is added
// only when some peer ID appears on more than one row: connections sharing a
// peer ID belong to one node, and algod reports that node's traffic totals on
// each of them. The totals are printed on the first such row only and zeroed
// on the rest, so that summing the column gives the right answer, and a footer
// says so. Being last, the column adds nothing to rows that do not need it.
func printPeers(w io.Writer, peers []model.PeerStatus, verbose bool) {
	repeated := repeatedPeerIDs(peers)

	type row struct {
		received, sent, streams, peerID string
	}
	rows := make([]row, len(peers))
	addressWidth, streamsWidth := len("ADDRESS"), len("STREAMS")
	counted := make(map[string]bool, len(repeated))
	for i, peer := range peers {
		r := row{received: "-", sent: "-", streams: "-"}
		if peer.TotalBytesReceived != nil {
			r.received = formatByteCount(*peer.TotalBytesReceived)
		}
		if peer.TotalBytesSent != nil {
			r.sent = formatByteCount(*peer.TotalBytesSent)
		}
		if peer.ActiveStreams != nil && len(*peer.ActiveStreams) > 0 {
			r.streams = friendlyStreams(*peer.ActiveStreams)
		}
		if peer.PeerId != nil && repeated[*peer.PeerId] {
			r.peerID = peerIDTail(*peer.PeerId)
			if counted[*peer.PeerId] {
				// this node's totals were already printed on an earlier row
				if peer.TotalBytesReceived != nil {
					r.received = formatByteCount(0)
				}
				if peer.TotalBytesSent != nil {
					r.sent = formatByteCount(0)
				}
			}
			counted[*peer.PeerId] = true
		}
		rows[i] = r
		addressWidth = max(addressWidth, len(peer.NetworkAddress))
		streamsWidth = max(streamsWidth, len(r.streams))
	}

	// the STREAMS column is padded only when a column follows it
	rowFormat := "%-9s  %-7s  %-*s  %9s  %9s  %-*s"
	if len(repeated) == 0 {
		streamsWidth = 0
	} else {
		rowFormat += "  %s"
	}
	rowFormat += "\n"
	printRow := func(connType, netType, addr, received, sent, streams, peerID string) {
		if len(repeated) == 0 {
			fmt.Fprintf(w, rowFormat, connType, netType, addressWidth, addr, received, sent, streamsWidth, streams)
			return
		}
		fmt.Fprintf(w, rowFormat, connType, netType, addressWidth, addr, received, sent, streamsWidth, streams, peerID)
	}

	printRow("CONN TYPE", "NETWORK", "ADDRESS", "RECEIVED", "SENT", "STREAMS", "PEER")
	for i, peer := range peers {
		r := rows[i]
		printRow(string(peer.ConnectionType), string(peer.NetworkType), peer.NetworkAddress, r.received, r.sent, r.streams, r.peerID)
		if !verbose {
			continue
		}
		if peer.PeerId != nil && *peer.PeerId != "" {
			printPeerDetail(w, "peer", sanitizeTerminal(*peer.PeerId))
		}
		if peer.SupportedProtocols != nil {
			// one per line: a node supports a dozen or more protocol IDs,
			// which joined on one line wrap badly on most terminals
			printPeerDetail(w, "supported protocols", "")
			for _, proto := range *peer.SupportedProtocols {
				printPeerDetailItem(w, sanitizeTerminal(proto))
			}
		}
	}
	if len(repeated) > 0 {
		fmt.Fprintf(w, "%d peer ID(s) with more than one connection: traffic totals cover all of a peer's connections and are shown on its first row only\n", len(repeated))
	}
}

// printPeerDetail prints a continuation line under a peer row, indented past
// the CONN TYPE and NETWORK columns.
func printPeerDetail(w io.Writer, label, value string) {
	if value == "" {
		fmt.Fprintf(w, "%-9s  %-7s  %s:\n", "", "", label)
		return
	}
	fmt.Fprintf(w, "%-9s  %-7s  %s: %s\n", "", "", label, value)
}

// printPeerDetailItem prints one entry of a list opened by printPeerDetail.
func printPeerDetailItem(w io.Writer, value string) {
	fmt.Fprintf(w, "%-9s  %-7s    %s\n", "", "", value)
}

// repeatedPeerIDs returns the peer IDs that appear on more than one of the
// given peers. Each is a node the host holds several connections to.
func repeatedPeerIDs(peers []model.PeerStatus) map[string]bool {
	seen := make(map[string]bool)
	repeated := make(map[string]bool)
	for _, peer := range peers {
		if peer.PeerId == nil || *peer.PeerId == "" {
			continue
		}
		if seen[*peer.PeerId] {
			repeated[*peer.PeerId] = true
		}
		seen[*peer.PeerId] = true
	}
	return repeated
}

// peerIDDisplayLen is how many trailing characters of a peer ID are shown in
// the PEER column. Peer IDs derived from Ed25519 keys all begin "12D3KooW",
// so it is the tail that tells them apart.
const peerIDDisplayLen = 8

// peerIDTail renders a peer ID for the PEER column: its last few characters,
// marked as cut when the ID is longer than that. The ID is escaped first and
// cut on runes, so that the cut cannot land inside an escape.
func peerIDTail(peerID string) string {
	id := []rune(sanitizeTerminal(peerID))
	if len(id) > peerIDDisplayLen {
		id = append([]rune(".."), id[len(id)-peerIDDisplayLen:]...)
	}
	return string(id)
}

// streamNames maps stream protocol ID prefixes to short display names, listed
// in display order. pubsub comes before gossip: on a p2p network nearly every
// connection carries pubsub while only some carry gossip, so this order keeps
// the column reading consistently down a table. Then the rest of the
// application traffic, then libp2p machinery.
var streamNames = []struct{ prefix, name string }{
	{"/meshsub/", "pubsub"},
	{"/algorand-ws/", "gossip"},
	{"ws-gossip/", "gossip"},
	{"/algorand/kad/", "dht"},
	{"/http/", "http"},
	{"/ipfs/id/", "identify"},
	{"/ipfs/ping/", "ping"},
}

// friendlyStreams renders stream protocol IDs as short names, deduplicated and
// in streamNames order. A stream whose protocol is still being negotiated has
// an empty ID and is shown as "negotiating", after the known names.
// Unrecognized protocols are shown verbatim, last.
func friendlyStreams(protos []string) string {
	ranks := make(map[string]int, len(protos))
	names := make([]string, 0, len(protos))
	for _, proto := range protos {
		name, rank := sanitizeTerminal(proto), len(streamNames)+1
		if proto == "" {
			name, rank = "negotiating", len(streamNames)
		}
		for i, s := range streamNames {
			if strings.HasPrefix(proto, s.prefix) {
				name, rank = s.name, i
				break
			}
		}
		if _, seen := ranks[name]; !seen {
			ranks[name] = rank
			names = append(names, name)
		}
	}
	slices.SortFunc(names, func(a, b string) int {
		if d := ranks[a] - ranks[b]; d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	return strings.Join(names, ",")
}

// sanitizeTerminal makes s safe to print to a terminal. Protocol names reach
// us from the remote peer, which is free to put control characters, newlines
// or ANSI/OSC escape sequences in them; every rune that is not printable is
// replaced by its escape so a peer cannot rewrite the operator's display.
func sanitizeTerminal(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1: // not valid UTF-8, escape the byte
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case unicode.IsPrint(r):
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
		i += size
	}
	return b.String()
}

// formatByteCount renders a byte count in a compact human-readable form.
func formatByteCount(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the current node status",
	Long:  `Show the current status of the running Algorand node.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		datadir.OnDataDirs(getStatus)
	},
}

func getStatus(dataDir string) {
	const (
		CUU = string("\033[A") // Cursor Up
		DL  = string("\033[M") // Delete Line
	)
	client := ensureAlgodClient(dataDir)
	cleanupFmt := ""
	for {
		stat, err := client.Status()
		if err != nil {
			reportErrorf(errorNodeStatus, err)
		}
		vers, err := client.AlgodVersions()
		if err != nil {
			reportErrorf(errorNodeStatus, err)
		}
		status := cleanupFmt + makeStatusString(stat) + "\n"
		if vers.GenesisID != "" {
			status = fmt.Sprintf("%sGenesis ID: %s\n", status, vers.GenesisID)
		}
		status = fmt.Sprintf("%sGenesis hash: %s", status, base64.StdEncoding.EncodeToString(vers.GenesisHash[:]))
		fmt.Println(status)
		if watchMillisecond == 0 {
			break
		}
		time.Sleep(time.Duration(watchMillisecond) * time.Millisecond)
		cleanupFmt = ""
		for linesCount := len(strings.Split(status, "\n")); linesCount > 0; linesCount-- {
			cleanupFmt += CUU + DL
		}
	}
}

func makeStatusString(stat model.NodeStatusResponse) string {
	lastRoundTime := fmt.Sprintf("%.1fs", time.Duration(stat.TimeSinceLastRound).Seconds())
	catchupTime := fmt.Sprintf("%.1fs", time.Duration(stat.CatchupTime).Seconds())
	var statusString string

	if stat.Catchpoint == nil || (*stat.Catchpoint) == "" {
		statusString = fmt.Sprintf(
			infoNodeStatus,
			stat.LastRound,
			lastRoundTime,
			catchupTime,
			stat.LastVersion,
			stat.NextVersion,
			stat.NextVersionRound,
			stat.NextVersionSupported)

		if stat.LastCatchpoint != nil {
			statusString = statusString + "\n" + fmt.Sprintf(nodeLastCatchpoint, *stat.LastCatchpoint)
		}

		if stat.StoppedAtUnsupportedRound {
			statusString = statusString + "\n" + fmt.Sprintf(catchupStoppedOnUnsupported, stat.LastRound)
		}

		upgradeNextProtocolVoteBefore := nilToZero(stat.UpgradeNextProtocolVoteBefore)

		if upgradeNextProtocolVoteBefore > stat.LastRound {
			upgradeVotesRequired := nilToZero(stat.UpgradeVotesRequired)
			upgradeNoVotes := nilToZero(stat.UpgradeNoVotes)
			upgradeYesVotes := nilToZero(stat.UpgradeYesVotes)
			upgradeVoteRounds := nilToZero(stat.UpgradeVoteRounds)
			statusString = statusString + "\n" + fmt.Sprintf(
				infoNodeStatusConsensusUpgradeVoting,
				upgradeYesVotes,
				upgradeNoVotes,
				upgradeVoteRounds-upgradeYesVotes-upgradeNoVotes,
				upgradeVotesRequired,
				upgradeNextProtocolVoteBefore,
			)
		} else if upgradeNextProtocolVoteBefore > 0 {
			statusString = statusString + "\n" + infoNodeStatusConsensusUpgradeScheduled
		}

	} else {
		statusString = fmt.Sprintf(
			infoNodeCatchpointCatchupStatus,
			stat.LastRound,
			catchupTime,
			*stat.Catchpoint)

		if stat.CatchpointTotalAccounts != nil && (*stat.CatchpointTotalAccounts > 0) && stat.CatchpointProcessedAccounts != nil {
			statusString = statusString + "\n" + fmt.Sprintf(infoNodeCatchpointCatchupAccounts, *stat.CatchpointTotalAccounts,
				*stat.CatchpointProcessedAccounts, *stat.CatchpointVerifiedAccounts,
				*stat.CatchpointTotalKvs, *stat.CatchpointProcessedKvs, *stat.CatchpointVerifiedKvs)
		}
		if stat.CatchpointAcquiredBlocks != nil && stat.CatchpointTotalBlocks != nil && (*stat.CatchpointAcquiredBlocks+*stat.CatchpointTotalBlocks > 0) {
			statusString = statusString + "\n" + fmt.Sprintf(infoNodeCatchpointCatchupBlocks, *stat.CatchpointTotalBlocks,
				*stat.CatchpointAcquiredBlocks)
		}
	}

	return statusString
}

var lastroundCmd = &cobra.Command{
	Use:   "lastround",
	Short: "Print the last round number",
	Long:  `Prints the most recent round confirmed by the Algorand node.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		datadir.OnDataDirs(func(dataDir string) {
			round, err := ensureAlgodClient(dataDir).CurrentRound()
			if err != nil {
				reportErrorf(errorNodeStatus, err)
			}

			reportInfof("%d\n", round)
		})
	},
}

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone the specified node to create another node",
	Long:  `Clone the specified node to create another node. Optionally you can control whether the clone includes the current ledger, or if it starts with an uninitialized one. The default is to clone the ledger as well. Specify -n or --noledger to start with an uninitialized ledger.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		binDir, err := util.ExeDir()
		if err != nil {
			panic(err)
		}
		nc := nodecontrol.MakeNodeController(binDir, datadir.EnsureSingleDataDir())
		err = nc.Clone(targetDir, !noLedger)
		if err != nil {
			reportErrorf(errorCloningNode, err)
		} else {
			reportInfof(infoNodeCloned, targetDir)
		}
	},
}

// Simple command to dump a snapshot of current pending transactions in the node's transaction pool
var pendingTxnsCmd = &cobra.Command{
	Use:   "pendingtxns",
	Short: "Get a snapshot of current pending transactions on this node",
	Long:  `Get a snapshot of current pending transactions on this node, cut off at MAX transactions (-m), default 0. If MAX=0, fetches as many transactions as possible.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		datadir.OnDataDirs(func(dataDir string) {
			client := ensureAlgodClient(dataDir)
			statusTxnPool, err := client.GetParsedPendingTransactions(maxPendingTransactions)
			if err != nil {
				reportErrorf(errorNodeStatus, err)
			}

			pendingTxns := statusTxnPool.TopTransactions

			// do this inline for now, break it out when we need to reuse a Txn->String function
			reportInfof(infoNodePendingTxnsDescription, maxPendingTransactions, statusTxnPool.TotalTransactions)
			if len(statusTxnPool.TopTransactions) == 0 {
				reportInfof(infoNodeNoPendingTxnsDescription)
			} else {
				for _, pendingTxn := range pendingTxns {
					pendingTxnStr, err := json.MarshalIndent(pendingTxn, "", "    ")
					if err != nil {
						// json parsing of the txn failed, so let's just skip printing it
						fmt.Printf("Unparseable Transaction %s\n", pendingTxn.Txn.ID().String())
						continue
					}
					fmt.Printf("%s\n", string(pendingTxnStr))
				}
			}
		})
	},
}

var waitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Waits for the node to make progress",
	Long:  "Waits for the node to make progress, which includes catching up.",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {
		client := ensureAlgodClient(datadir.EnsureSingleDataDir())
		stat, err := client.Status()
		if err != nil {
			reportErrorf(errorNodeStatus, err)
		}

		startRound := stat.LastRound
		endTime := time.After(time.Second * time.Duration(waitSec))
		for {
			select {
			case <-endTime:
				reportErrorf("Timed out waiting for node to make progress")
			case <-time.After(500 * time.Millisecond):
				stat, err = client.Status()
				if err != nil {
					reportErrorf(errorNodeStatus, err)
				}
				if startRound != stat.LastRound {
					os.Exit(0)
				}
			}
		}
	},
}

func isValidIP(userInput string) bool {
	host, port, err := net.SplitHostPort(userInput)
	if err != nil {
		return false
	}
	if port == "" {
		return false
	}
	if host == "" {
		return false
	}
	return net.ParseIP(host) != nil
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a node at the desired data directory for the desired network",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, _ []string) {

		// validate network input
		validNetworks := map[string][]byte{
			"mainnet": genesisMainnet,
			"testnet": genesisTestnet,
			"betanet": genesisBetanet,
			"devnet":  genesisDevnet,
		}
		var genesisContent []byte
		var ok bool
		if genesisContent, ok = validNetworks[newNodeNetwork]; !ok {
			reportErrorf(errorNodeCreation, "passed network name invalid")
		}

		// validate and store passed options
		localConfig := config.GetDefaultLocal()
		if newNodeRelay != "" {
			if isValidIP(newNodeRelay) {
				localConfig.NetAddress = newNodeRelay
			} else {
				reportErrorf(errorNodeCreationIPFailure, newNodeRelay)
			}
		}
		if listenIP != "" {
			if isValidIP(listenIP) {
				localConfig.EndpointAddress = listenIP
			} else {
				reportErrorf(errorNodeCreationIPFailure, listenIP)
			}
		}
		localConfig.Archival = newNodeArchival || newNodeRelay != ""
		localConfig.RunHosted = runUnderHost
		localConfig.EnableLedgerService = localConfig.Archival
		localConfig.EnableBlockService = localConfig.Archival

		// verify destination does not exist, and attempt to create destination folder
		if util.FileExists(newNodeDestination) {
			reportErrorf(errorNodeCreation, "destination folder already exists")
		}
		destPath := filepath.Join(newNodeDestination, "genesis.json")
		err := os.MkdirAll(newNodeDestination, 0766)
		if err != nil {
			reportErrorf(errorNodeCreation, "could not create destination folder")
		}

		// copy genesis block to destination
		err = os.WriteFile(destPath, genesisContent, 0644)
		if err != nil {
			reportErrorf(errorNodeCreation, err)
		}

		// save config to destination
		if newNodeFullConfig {
			err = localConfig.SaveAllToDisk(newNodeDestination)
		} else {
			err = localConfig.SaveToDisk(newNodeDestination)
		}
		if err != nil {
			reportErrorf(errorNodeCreation, err)
		}
	},
}

// verifyPeerDialArg verifies that the peers provided in peerDial are valid peers.
func verifyPeerDialArg() bool {
	if peerDial == "" {
		return true
	}

	// make sure that the format of each entry is valid:
	for peer := range strings.SplitSeq(peerDial, ";") {
		_, err := naddr.ParseHostOrURLOrMultiaddr(peer)
		if err != nil {
			reportErrorf("Provided peer '%s' is not a valid peer address : %v", peer, err)
			return false
		}

	}
	return true

}
