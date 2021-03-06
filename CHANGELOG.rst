
v0.85.1 (unreleased)
--------------------

- Added ``MaxAppendRate`` JournalSpec field and global broker flag.
  Append RPCs now use a token-bucket flow control strategy, where RPC chunks
  are evaluated and potentially throttled or policed against maximum and minimum
  allowed flow-rates.
- Added ``PathPostfixTemplate`` JournalSpec field. Path postfixes are evaluated
  and applied to individual Fragments as they're persisted. A primary use case is
  to support Hive-compatible partitioning of Fragments based on their creation time.
- Reworked almost all documentation into reStructuredText / Sphinx / ReadTheDocs format.

v0.84.2
-------

- Add ca-certificates to release images.

v0.84.1
-------

- SQLite is now a supported `consumer store`_!
- Instances of message.Framing may now be dynamically registered. Support for ``text/csv`` is added.
- Added the ``gazctl attach-uuids`` sub-command.
- A bike-share_ example and documentation have been added,
  along with new kustomize_ manifests for deploying existing examples.
- Automated partition crash-tests are re-enabled, after adding
  DaemonSet kustomize manifests to properly support them.
- The ListFragments RPC now properly respects fragment stores which use re-write rules.
- The journals of ShardSpecs are now verified by consumer servers and ``gazctl``, to actually
  exist and have appropriate content-types.
- Extensive Godoc and documentation improvements.
- Various minor logging improvements and bug fixes.

.. _`consumer store`: https://godoc.org/go.gazette.dev/core/consumer/store-sqlite
.. _bike-share: docs/examples_bike_share.md
.. _kustomize: kustomize/test/
.. _Urkel: https://github.com/jgraettinger/urkel

v0.83.2
-------

This release introduces `exactly-once processing semantics`_ to Gazette!

This is a breaking change to many of the ``consumer`` package interfaces, notably Shard, Application and Store, as well as the ``message`` interfaces. Updates to consumer applications will be required.

This release also introduces Kustomize manifests for deploying brokers, consumers, and dependencies. *Helm charts of the repo are deprecated and will be removed in a future release*.

**Rolling Upgrades**

A rolling upgrade from v0.82 => v0.83 is supported and tested, with the following caveats:

- Brokers must be fully migrated to v0.83 before any consumers may be migrated. This is required
  as v0.83 brokers introduce journal "registers" which v0.83 consumers rely on. The v0.83 broker
  is fully compatible with v0.82 consumers.
- v0.83 consumers will migrate the means of storing offsets within RocksDB from now-legacy
  keys/values to new consumer Checkpoints introduced with v0.83.
  **Legacy offsets are not removed, but are also not updated.**
  This means downgrading from v0.83 => v0.82 will re-process portions of source journals read
  by the v0.83 consumer. Similarly, a subsequent re-upgrade from v0.82 => v0.83
  *will not migrate offsets again* (and portions read by the downgraded v0.82 consumer will
  be re-processed).

.. _`exactly-once processing semantics`: https://github.com/gazette/core/blob/master/docs/exactly_once_semantics.md

v0.82.2
-------

Release v0.82.2 is a patch release of the v0.82 branch

It includes fixes cherry-picked from master since v0.82.1 was cut:

- 36a01b6 consumer: fix some spurious shard recovery errors
- ac3a329 broker: add more context cancellation checks for log supression
- 35632e1 broker: proxyAppend should take AppendRequest by value (not reference)
- 4c6fa33 client: RouteCache should account for empty Route
- ef7098e allocator: update some logging
