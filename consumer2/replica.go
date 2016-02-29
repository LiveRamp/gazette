package consumer

import (
	log "github.com/Sirupsen/logrus"
	etcd "github.com/coreos/etcd/client"

	"github.com/pippio/gazette/recoverylog"
)

type replica struct {
	shard     ShardID
	player    *recoverylog.Player
	servingCh chan struct{} // Blocks until replica.serve exists.
}

func newReplica(shard *shard, runner *Runner, tree *etcd.Node) (*replica, error) {
	hints, err := loadHints(shard.id, runner, tree)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{"shard": shard.id, "hints": hints, "dir": shard.localDir}).
		Info("replicating with hints")

	player, err := recoverylog.PreparePlayback(hints, shard.localDir)
	if err != nil {
		return nil, err
	}
	player.SetCancelChan(shard.cancelCh)

	return &replica{
		shard:     shard.id,
		player:    player,
		servingCh: make(chan struct{}),
	}, nil
}

func (r *replica) serve(runner *Runner) {
	defer close(r.servingCh)

	err := r.player.Play(runner.Getter)

	if err != nil && err != recoverylog.ErrPlaybackCancelled {
		log.WithFields(log.Fields{"shard": r.shard, "err": err}).Error("replication failed")
		abort(runner, r.shard)
		return
	}
	log.WithFields(log.Fields{"shard": r.shard, "err": err}).Info("finished serving replica")
}
