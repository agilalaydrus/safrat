# Handoff Notes

> Working state + prioritized roadmap for the next agent. Point-in-time snapshot
> (2026-08-29). Verify against current code before trusting any file:line.

## Owner workflow preferences

- After verified implementation work, always create a local commit so progress
  is recorded. Never push or deploy unless the owner explicitly asks.
- After every commit, make sure the local development server is running again
  and verify the web endpoint so the owner can immediately inspect the result.
- Every handoff response must include: a concise summary, completed work,
  remaining/unverified work, recommendations, local commit, and server status.

## Continuation after this snapshot

> **Mulai dari [docs/STATUS.md](docs/STATUS.md)** — satu halaman berisi posisi
> terkini kedua jalur, kondisi terverifikasi, dan jebakan yang sudah menipu kami.
> Berkas ini adalah riwayat; STATUS.md adalah posisi.

### Panel SaaS — rencana lengkap (2026-09-02)

Panel pemilik platform (`/admin`). Sudah ada delapan permukaan berjalan dan 30
RPC; yang kurang adalah sisi komersialnya. Rancangan di
**`docs/RENCANA-PANEL-SAAS.md`**, tugas berurutan di
**`docs/TUGAS-PANEL-SAAS.md`**.

Tiga hal yang ditemukan saat merancang dan berlaku lintas pekerjaan:

1. **`plan_limits` dan `plan_overrides` tidak punya satu pun RPC.** T2.2
   menegakkan batas lewat trigger database, tapi mengubah kuota satu pelanggan
   hari ini berarti menulis SQL di produksi.
2. **Tiga RPC platform masih tanpa UI** — `ListProductRoutes`,
   `SaveProductRoute`, `ListSupplierLogs`. Routing produk hanya bisa diubah
   lewat terminal, padahal panel ini lahir untuk menghapus kebutuhan itu.
3. Panel ini satu-satunya permukaan yang **menembus batas tenant**, jadi
   isolasi cabang Tahap 2 tidak berlaku di sini. Impersonate, four-eyes, dan
   audit pembacaan data pribadi dijadwalkan sebelum admin kedua ditambahkan.

### Dashboard Admin Travel — rencana lengkap (2026-08-30)

Pekerjaan berikutnya yang sudah direncanakan penuh tapi **belum dimulai**.
Daftar tugas berurutan ada di **`docs/TUGAS-DASHBOARD-TRAVEL.md`** — mulai dari
sana, bukan dari berkas ini.

- `docs/BENCHMARK-MEEQOT.md` — analisa pesaing (Meeqot): rencana, harga, roadmap
- `docs/RENCANA-DASHBOARD-TRAVEL.md` — 22 rute mereka vs 27 menu kita
- `docs/DESAIN-DASHBOARD-TRAVEL.md` — 8 pola UI, sistem `tone`, resep visual
- `docs/referensi/meeqot/` — data mentah hasil ekstraksi bundle mereka

Dua hal yang ditemukan saat merencanakan dan berlaku lintas pekerjaan:

1. **Paket kita tidak membatasi apa pun** kecuali domain kustom. Setiap fitur
   baru bocor gratis ke STARTER sampai entitlement ada (T2.2).
2. **Primitif UI memakai inline style**, yang tidak bisa menyatakan `:hover`
   atau `:focus-visible`. Itu penyebab struktural tampilan terasa kaku, dan
   memblokir seluruh Tahap 0 (T0.1).


### Money paths — audit findings and backlog (2026-08-26)

Everything below was verified against the running code and database, not
inferred. Ordered by what costs the most while it stays broken.

#### Already fixed this session

| Commit | What was wrong |
| --- | --- |
| `fe436f0` | `RequestPayout` read balance then inserted with no lock — concurrent calls let an agent request **more than they are owed**. Now serialised per agent with an advisory lock; test drives 8 concurrent requests. |
| `1e22e9c` | Check-then-act let one operator hold two live subscription invoices, so two unique amounts were in play at once. Partial unique index; the losing request gets the winning invoice. |
| `b7e98ca` | Offline chat replay posted duplicate messages. Client-generated key per message, per-sender unique index, `DO UPDATE` so the row comes back. |
| `8b719e8` | Transfer amounts were only unique among *unpaid* invoices, so a settled code could be reissued the same day and a mutation would be ambiguous. Now unique per day (Asia/Jakarta) too. |
| `c5535f1` | `ExpireOverdueInvoices`/`MarkLapsed` existed but nothing called them — abandoned invoices held their unique code forever and the 999-suffix pool drained. Hourly sweep. |

#### Order flow — what is already correct

Do not "fix" these; they were checked:
- **Price comes from the server** (`product.PriceIDR`), never the request. The
  top payment vulnerability is absent.
- Margin split computed server-side; product fetched scoped to the caller's
  operator; identity taken from `app_access_code`, not a client-supplied id.
- Settlement is idempotent: `WHERE status = 'PENDING'`, so a redelivered
  webhook cannot pay twice, and `applyPaidSideEffects` cannot re-run.
- Webhook token verified; `xendit_invoice_id` has a unique index.

#### Owner requirements for the money paths (2026-08-26, verbatim intent)

These are decisions, not suggestions. Build to them.

1. **A refund returns the customer's balance/deposit, and reverses the agent
   commission** — commission is earned only on *successful* transactions, so a
   refunded order must claw it back.
   *Implied and currently missing:* there is **no customer balance or deposit
   concept anywhere** — no wallet, balance or ledger table exists. And
   commission is not a ledger: it is a column on `orders`
   (`agent_commission_idr`) summed over PAID orders, so there is nothing to
   reverse. Both need to become append-only ledgers before a refund can be
   expressed honestly. A reversal must be a new negative entry, never an edit
   of the original — an edited history cannot be audited.

2. **The same idempotency key always means an *advice* about the same
   transaction**, never a new one. A repeat must return the original outcome,
   not merely avoid a duplicate insert. That is stronger than a unique index:
   the key and its response have to be **stored**, so a replay after the
   original response was lost replays that response — same id, same status,
   same amount. Applies to every money-moving endpoint, not only the ones fixed
   so far.

3. **Structure follows the product type:**
   - *Physical* — proper e-commerce structure: stock, address, shipment,
     tracking, delivery state, proof of handover.
   - *Digital* — follow the wallet/e-money pattern (DANA, GoPay): issue,
     deliver, and settle with an explicit fulfilment state, provider reference,
     and reversal path.
   - *Umroh packages* — follow established industry best practice rather than
     inventing a flow.

4. **The paid amount must always be validated**, by the best available method —
   not assumed correct because the provider said PAID.

4. **Fraud and suspect handling is required**, including a held/suspended
   ("gantung") state so a questionable transaction is neither settled nor
   rejected while it is reviewed, and is excluded from totals meanwhile.

5. **Receipts are per transaction and per account that transacted.** The person
   who paid must be able to preview and print their own receipt — not only the
   operator.

#### Done — PR 1: money ledgers (migration 090)

Requirement 1's precondition. Two append-only ledgers now exist:
`agent_commission_entries` and `pilgrim_balance_entries`.

- **Append-only is enforced by the database**, not by convention: a trigger
  rejects every UPDATE, and every DELETE unless `app.allow_ledger_purge` is set
  for the transaction. That flag exists only so an operator/tenant teardown (and
  test fixtures) can still cascade; ordinary code paths cannot reach it.
- **A reversal is a new negative row** (`kind = 'REVERSED'`), so a balance can
  return to zero while both the earning and the clawback stay visible.
- **`agents.id` is referenced with ON DELETE RESTRICT.** An agent who has earned
  commission can no longer be deleted; `AgentService.Delete` maps that FK
  violation to a clear Indonesian message telling the operator to deactivate
  instead. Deleting such an agent would have destroyed the payout history.
- **Idempotent by index, not by check-then-act**: unique on
  `(agent_id, idempotency_key)` and on `(order_id, kind)`. `AppendCommission`/
  `AppendBalance` treat those two violations as success, so a redelivered
  webhook credits once. Verified with six concurrent appends of one key →
  balance credited exactly once.
- **Reads switched over**: `GetAgentPayoutSummary` and `ListAgentPayouts` now
  read commission from the ledger instead of summing PAID orders, so a future
  reversal will actually reduce what an agent can withdraw.
- **Writes wired in**: `applyPaidSideEffects` appends an `EARNED` entry keyed by
  the order. Failure is reported to Sentry rather than returned — the payment is
  already settled, and erroring would make the provider retry a success.
- Existing balances were preserved by a backfill from PAID orders, so no agent's
  payable figure changed at the cutover.

Still open in this area: nothing writes `pilgrim_balance_entries` yet — it is
the target of the refund PR below, which is what puts money into it.

#### Done — PR 2: refunds (migration 091)

Requirement 1, in full. `orders.status` now allows `REFUNDED`, and
`order_refunds` records every refund event.

- **`OrderService.RefundOrder`** records a refund, credits the pilgrim's
  balance ledger, and reverses the agent's commission — all in one transaction
  under `SELECT ... FOR UPDATE` on the order. A half-applied refund would
  credit a pilgrim while leaving the agent paid for a sale that no longer
  exists.
- **A refund is always the whole transaction** (owner's rule, migration 093).
  The order goes straight to `REFUNDED`, the pilgrim is credited what they
  paid, and the commission is reversed in full. A non-`PAID` order cannot be
  refunded at all.
- **`idempotency_key` is required on the RPC.** A replay returns the refund that
  already exists with `created = false`; six concurrent calls with one key
  produced one refund row and credited the pilgrim once.
- **Conflicts are declined, never raised.** `ON CONFLICT DO NOTHING` replaced
  catch-the-unique-violation in both the refund insert and the ledger appends: a
  failed statement poisons the whole transaction, so the recovery read that
  fetches the existing row would itself fail. This was found by the concurrency
  test, not by review.
- **UI**: `components/orders/RefundOrderDialog.tsx`, opened from the Refund
  action on paid orders. The idempotency key is minted once per refund the
  operator is composing, so a double-click carries the same key.

It records a refund; it does not call the gateway to move money. Operators
refund by transfer or at the counter today, and an honest record is the part
that has to exist first.

#### Fixed alongside — two red tests

- **Transport transition fixtures.** `TestMovementAndVehicleTransitions` and
  `TestDeleteMovement` failed on `cannot transition from scheduled to arrived`.
  Pre-existing and unrelated to the ledgers: `arrived` is only reachable through
  `departed` (see `transitionAllowed`), but the fixtures jumped straight to it,
  so the setup was rejected by the very rule the tests exist to check. They now
  walk the legal path via `transitionPath`. Writing the status with a direct
  UPDATE would also have made them pass, while quietly ending any proof that the
  legal path is walkable.
- **An inverted assertion I introduced** in the ledger commit: renaming
  `isUniqueViolation` to `IsUniqueViolation` dropped the `!` in
  `TestSubscriptionTransferAmountIsUniquePerDay`, so a correct rejection was
  reported as the wrong reason. Every other renamed call site was checked; only
  that one was affected.

#### Done — PR 3: net payments and the refund invariant (migration 092)

Found while assessing the risk of shipping PR 2, and fixed before deploying it.

**The defect.** "How much has this pilgrim paid" was read as
`SUM(orders.total_price_idr) WHERE status = 'PAID'`. That was true until money
could go back. A partially refunded order stays `PAID` at its full price, so the
figure overstated by exactly the amount refunded — and
`CancellationService.computePreview` multiplies it by the policy's refund
percentage. **The operator would have refunded the same money a second time.**
`GetSeasonOrderStats` and `GetSeasonPaidTotal` overstated revenue the same way.

**The fix is one definition, not three patches.** `order_payments` is now the
single answer to that question, and it is shaped so that misusing it is hard:
`net_paid_idr` is already zero for orders that were never paid, so a caller
needs no status filter and cannot forget one. A fully refunded order reaches
zero by arithmetic rather than by its status, so the two can never disagree.
All three queries read it. Any future query asking "how much was paid" should
too — patching the three call sites would only have left the fourth to repeat
the mistake.

**The invariant is now the database's, not the service's.** A trigger on
`order_refunds` rejects a refund that would push the running total past what
was paid, and rejects refunding an order that was never paid — taking
`FOR UPDATE` on the order itself, so it holds even for a caller that takes no
lock. The service still checks first, because that is where a useful message
comes from; the trigger is what makes the rule true of the data. Verified by
writing straight to the table, bypassing the service entirely.

**Commission reconciliation is automatic** (`worker/commission.go`, every 10
minutes). Deploy runs migrations *before* restarting the API, so an order paid
in that window is marked PAID by a binary that predates the ledger and no later
event ever revisits it — the agent is never credited, silently and permanently.
The sweep is one set-based statement keyed by the same idempotency key the
payment path uses: it adds only what is missing, never removes or adjusts, and
leaves a refund's reversal alone rather than "restoring" the earning. Running it
twice changes nothing. This replaces the manual post-deploy SQL that a human
would eventually forget.

**Still open from the same review:** a pilgrim's refunded balance is credited to
`pilgrim_balance_entries`, but nothing reads it except the refund response —
no pilgrim-facing display, no way to spend it against another order, no
withdrawal path. And `RefundOrder` records a refund without moving money at the
gateway, so an operator can record one and forget to transfer.

#### Done — PR 4: refunds are never partial (migration 093)

Owner's rule, stated directly: *"Refund transaksi tidak pernah boleh sebagian
dari nilai transaksi."* PR 2 had allowed partial refunds; that is now not a
thing the system can represent.

- **The RPC carries no amount.** `RefundOrderRequest.amount_idr` is removed and
  the field number reserved. A partial refund is not rejected by a check — it
  cannot be expressed. The service takes the amount from the order.
- **The database agrees, for callers that never reach the service.** The
  trigger from 092 now requires `amount_idr` to equal the order's total exactly,
  and a unique index allows one refund per order. Verified by writing straight
  to the table: half the total rejected, more than the total rejected, the exact
  total accepted once and refused the second time.
- **The ledger indexes tightened back up.** 091 had loosened them precisely so a
  partial refund could reverse commission repeatedly. That reason is gone, so
  `(order_id, kind)` uniqueness returns for `EARNED`/`REVERSED` and
  `PURCHASE`/`REFUND`. `ADJUSTMENT` stays unconstrained — a manual correction
  is not a transaction event and may legitimately repeat.
- **Commission reversal is simply the whole commission.** All the pro-rata
  rounding logic is gone.

**A contract violation this surfaced.** Because a refund now leaves the order
`REFUNDED`, the status precondition started rejecting *replays*: an operator
who never saw the first response and pressed the button again got "only paid
orders can be refunded", and would conclude the refund failed when it had
succeeded. The idempotency lookup now runs **before** any precondition — a
replay must not be judged against the state its own original request created.
Caught by the concurrency test, not by review.

#### Verified — the refund path over real HTTP

`internal/handler/order_refund_http_test.go` drives a real `httptest` server
with a real Connect client, so the layers a service-level test cannot reach are
now covered: the Connect handler, protovalidate, the auth interceptor's session
lookup, the subscription gate, and the wire contract. It mints its own Better
Auth user/org/member/session in the test database and removes them afterwards —
no real session token is ever read.

Confirmed: unauthenticated calls get `unauthenticated`; a missing idempotency
key and a malformed order id are rejected by **protovalidate**, before any
business logic; the refund succeeds over the wire with the order `REFUNDED` and
the pilgrim credited; a replay returns the original refund with
`created = false`; `ListOrderRefunds` reports exactly one; and **a lapsed
subscription locks the refund endpoint** with `failed_precondition` — the gate
lives in the interceptor, so nothing below HTTP could have shown that.

#### Done — PR 5: transaction history for jamaah, muttawwif and agent

Owner's request: a transaction history page for jamaah and muttawwif, and a
transaction recap in the agent portal covering customers under their referral.

**A gap our own refund work created.** `ListOrderCreditsForAgent` read PAID
orders, so a refunded order left the agent's transaction list entirely while
their balance dropped by the same amount — money vanishing with no line
explaining it. The wallet now reads `agent_commission_entries`, so the earning
and its reversal both appear and the list accounts for the balance.
`WalletTransactionType` gained `REVERSAL` and `ADJUSTMENT` so a clawback cannot
be mistaken for a credit.

- **Jamaah** — `/pilgrim/transactions`. Their orders, what was paid, what came
  back, and the balance the operator holds for them. A refunded transaction
  stays in the list, struck through and labelled, rather than disappearing:
  somebody whose money was returned needs to see that it was. A pending one
  still carries its payment link.
- **Muttawwif** — `/leader/transactions`. The commission ledger, each entry
  showing its direction and reason.
- **Agent** — "Rekap Transaksi" tab. One row per referred jamaah: orders, what
  is still held after refunds, and the commission it produced. All net —
  `order_payments` for money, the ledger for commission.

**Security decision on the jamaah endpoint.** `ListMyTransactions` is in
`sessionOnlyProcedures`, **not** `publicProcedures` where the rest of
PilgrimAppService lives. `app_access_code` also opens the schedule and product
list and travels through links and caches; payment history is a step up. It
requires a valid Better Auth session **and** that the presented code belongs to
that session's own pilgrim (constant-time compare), and takes the pilgrim id
from the session rather than the request. Verified over real HTTP: the code
alone is refused, another account's session presenting this code is refused, the
jamaah's own session with a wrong code is refused, and only both together work.

#### Done — PR 6: referral attribution and order idempotency (migration 094)

**The referral system was not running at all on the main purchase path.** Both
`CreateOrder` and `CreateManualOrder` hard-coded `agentCommission = 0` and an
empty `agent_id`. `pilgrims.agent_id` existed and drove the referral *lists*,
but never any money: an agent's referral earned nothing the moment the jamaah
bought anything. `products.agent_margin_pct` was configured, validated, and
never read.

Commission now follows the referral on every lane — jamaah self-checkout, staff
manual order, and the new agent lane — via `computeSplit(product, quantity,
agentID)`.

**Selling is open; earning is not.** Owner's rule: anyone may transact for any
jamaah, not only their own referrals. `CreateOrderForPilgrim` lets an agent or
Muttawwif sell to any jamaah of their operator, and the commission still goes
to that jamaah's referrer. The seller is recorded separately as
`orders.placed_by_agent_id`. Keeping "who sold it" and "who earns from it"
apart in the data is what lets selling be free without letting anyone take a
commission that belongs to whoever brought the jamaah in. Verified: agent B
sells to agent A's jamaah → A is credited 600,000, B is credited 0, and
`placed_by_agent_id` is B.

A jamaah with no referrer produces no commission. Making the seller the
referrer would let an agent claim an unreferred jamaah by selling to them,
quietly and permanently. **If the owner wants the seller to earn in that case,
this is the decision to revisit.**

**Order creation is now idempotent** (backlog item 1). `orders.idempotency_key`
with a partial unique index per operator; the key is required by the schema on
all three lanes. A replay returns the existing order *and its existing checkout
link* rather than creating a second Xendit invoice the jamaah could also pay.
Verified with six concurrent manual orders under one key → one order, one
commission entry.

`payment.Client.Configured()` is now nil-safe: an unconfigured deployment
leaves the client nil, and a checkout request panicked rather than reporting
that payments are unavailable.

**A test that passed vacuously.** The first version of the agent-selling test
read the order row inside `if err == nil`, and since Xendit is unconfigured in
tests the order was never created — so every assertion was skipped and the test
was green while proving nothing. `payment.NewClientWithEndpoint` now lets tests
drive the real invoice path against a stub.

#### Constraint — TawafiqHub is the merchant for digital products

Owner, 2026-08-26: *"clients travel umroh semuanya bukan penjual untuk product
digital, yang punya jalur API ke supplier, dll hanya pihak tawafiqhub."*

Travel operators are a sales channel for digital products, not the merchant.
Only the platform holds supplier integrations. `platform_margin_pct` is the
platform's cut on that supply; the operator and agent margins are channel
margin.

**This contradicts the current schema.** `products` are per-operator
(`products.operator_id`, `products.season_id`), so today every operator creates
their own `ROAMING_DATA`/`PPOB_CREDIT` rows with prices they invent, and the
platform has no catalogue at all. Nothing built so far depends on operators
being the merchant, so nothing here is wrong — but the digital catalogue needs
to become platform-owned before digital products can actually be sold.

Related and still open: `PPOB_CREDIT` has **no provider integration whatsoever**
— a jamaah pays and no credit is ever sent. That fulfilment is the platform's
job, not the operator's, and it should stay disabled until it exists.

#### Done — PR 7: pending transactions count (migration 095)

Owner's rules, stated together:

> *"Transaksi itu bukan tentang siapa penjual siapa pembeli, semuanya
> berposisi sebagai pembeli disini. Tetapi jika jamaah tersebut punya master
> refferal diatasnya, akan otomatis memberikan komisi ke refferal nya tersebut
> kalau transaksi sukses saja."*
>
> *"transaksi pending atau terproses itu berarti sementara sudah di anggap
> terhitung semua yang terkait, kecuali gagal atau refund."*

**Commission is now recognised when the transaction is placed, not when it
settles.** Previously it was appended in `applyPaidSideEffects`, so a referrer
saw nothing until payment cleared. It is appended at order creation on all
three lanes, and taken back only by failure or refund.

**Recognised and settled are two different questions**, and conflating them
would let an agent withdraw money for a transaction nobody has paid for.
`agent_commission_state` (migration 095) answers both from the same ledger:

- `recognised_idr` — everything earned, pending included. What the agent has
  made, and what `total_earned_idr` reports.
- `settled_idr` — only what sits behind a completed transaction. The only
  figure a payout may draw on; `OutstandingIDR` is now `settled - disbursed`,
  not `recognised - disbursed`.
- `pending_idr` — the difference, surfaced in the wallet and on the muttawwif
  page so the two figures do not simply disagree with no explanation.

Neither is stored. Both are the ledger read two ways, so they cannot drift from
each other or from the transactions underneath.

**Failure reverses.** `EXPIRED`/`FAILED` now route through
`OrderService.MarkStatusByInvoiceID` rather than the repository, so the
reversal cannot be skipped by a caller that only knows about the status.

Verified: a pending checkout recognises 600,000 with settled and outstanding
both 0; paying it moves the whole amount to payable **without a second ledger
entry** — settling is a change of transaction state, not a new earning; and an
expired transaction returns to 0 while keeping both the earning and its
reversal on the record.

The reconciliation sweep now covers `PENDING` orders too, since that is when
commission is recognised.

**Framing correction:** the seller/buyer model was wrong. Everyone transacts as
a buyer; commission flows to the master referral above the buyer. Nothing in
the data assumed otherwise — `placed_by_agent_id` records who entered the
transaction, never who earns from it — but the language is corrected throughout.

**This is now complete:** migration 110 introduced the agent buyer and the
production lane was completed with migration 112; see "agent self-purchase"
below. The later owner ruling superseded the old referral assumption: an agent
buying for themselves earns no commission, including for the agent above them.

#### Done — PR 8: payment amount validation and the held state (migration 096)

Owner's rule: *"komisi tetap terhitung asalkan dia bayar ke server dengan harga
yg sesuai."* Backlog items 3 (amount validation) and part of 4 (suspect
handling).

**The webhook settled on the gateway's word that *something* was paid**, never
checking the amount. Revenue, commission and the jamaah's payment history all
followed from that word. `OrderService.SettlePayment` now compares the reported
amount against the order total before anything settles.

**A mismatch is held, not rejected.** `HELD` is a new order status: money did
arrive, so rejecting would strand a real payment, and settling would accept an
amount nobody agreed to. A held transaction still counts as pending — it
neither failed nor was refunded — so the commission stays recognised, but it
cannot settle and therefore cannot be paid out. `net_paid_idr` is zero for it,
so it is not revenue either.

`orders.paid_amount_idr` keeps what the gateway reported, on settled orders as
well as held ones: a settled order now carries the evidence the check was made,
not just the claim.

**An unreported amount is treated as unverifiable, not as a match.** If a
delivery carries neither `paid_amount` nor `amount`, the order is held with a
reason that names it as a gateway/configuration fault rather than a suspicious
payer. This is the safe direction, but note it would hold *every* payment if
Xendit's field names ever changed — the held reason is written to be obvious
about that.

Verified: a payment one rupiah short is held with its reason and evidence,
commission stays counted but unpayable and net revenue is zero; a redelivered
notification with the *correct* amount does not rescue a held order; an
unreported amount is held; and a matching payment still settles and records
what was received.

Held orders surface as "Perlu Ditinjau" in the dashboard and "Sedang Diperiksa"
on the jamaah's page, in amber rather than red — the money is not lost, and the
jamaah is told so.

**Resolving a hold** landed in PR 12, below.

#### Done — PR 9: stop trusting the webhook

Owner: *"transaksi jangan pakai Webhook"*, plus the three webhook protections.

The gateway cannot be told to stop sending deliveries — Xendit only notifies
that way. What changed is that a delivery is no longer believed.

**Settlement now comes from Xendit's own API.** A delivery is demoted to a
hint — "look at this invoice" — and `OrderService.SettleFromGateway` fetches
the invoice over an authenticated outbound TLS call, then settles from that
answer. The delivery's `status` is not acted on, and its amount fields were
removed from the payload struct entirely so nothing can start using them again.

Why this matters more than the token: the callback token is a *static* shared
secret that rides on every delivery and sits in an env file. The outbound API
key never leaves the server. A forged delivery now buys an attacker one
pointless API call.

It also makes a dropped delivery survivable rather than silently fatal: the
same path is what a poller will call. **The poller itself is not built yet** —
today a missed delivery still leaves an order PENDING forever.

`FAILED` is the one exception, applied directly: Xendit keeps no failed invoice
to fetch, and that path only closes an order and reverses commission — the
direction that cannot pay anyone.

**On the three protections asked for:**

1. *Signature validation* — already present, and worth being precise about:
   Xendit's Invoice callbacks use a static `x-callback-token`, not a per-payload
   HMAC signature (that is Midtrans's `signature_key`). It is compared in
   constant time.
2. *Webhook idempotency* — already present: only `PENDING` orders move, so a
   redelivery is a no-op rather than a second settlement.
3. *IP allowlisting* — this one was genuinely missing. `WebhookSourceGuard`
   (`XENDIT_WEBHOOK_ALLOWED_IPS`, IPs or CIDRs) now refuses a delivery from an
   unexpected address before its body is read or its token compared.

**An empty allowlist permits everything, and warns loudly at startup.** Failing
closed would stop every payment settling the moment the variable is missing or
the gateway quietly changes its egress ranges — worse than a wider surface
sitting behind two stronger controls. The warning is there so it cannot become
invisible. **`XENDIT_WEBHOOK_ALLOWED_IPS` still needs setting on the VPS.**

#### Done — PR 10: settlement poller and webhook source allowlist

**The poller closes the last hole in the payment path.** `worker/payment.go`
sweeps every 2 minutes for orders that have been `PENDING` past a 5-minute
grace period with an invoice id, and settles them through
`OrderService.SettleFromGateway` — the same path the webhook uses, so there is
one definition of settlement and one place the amount is verified.

A dropped delivery is no longer permanent. Before this, a jamaah could pay and
the order would sit `PENDING` forever with nobody told.

The grace period exists so the poller does not race a notification that is
usually seconds away, and the sweep is bounded on both sides: batched at 100,
and it stops looking at invoices older than 7 days, which Xendit will never
resolve anyway. Verified: a fresh order is left alone, one aged past the grace
period is picked up and settles, and a settled order drops out of the queue
rather than being polled forever.

**Source allowlist.** `XENDIT_WEBHOOK_ALLOWED_IPS` is plumbed through
`.env.example`, `DEPLOY.md` and `docker-compose.prod.yml`, and edge blocks are
in place in both `deploy/nginx/safrat.conf` and `deploy/caddy/Caddyfile`.

**The addresses are deliberately left as placeholders, commented out.**
Checked against Xendit's documentation on 2026-08-27: **they do not publish
their webhook egress ranges at all** — the list has to be requested from Xendit
support. (Their dashboard's "IP Allowlist" is a different feature: it restricts
inbound API calls from you to them.) A wrong list at the edge blocks every
payment notification where the application-level guard cannot soften it. Once
support supplies the ranges, paste into all three places, `nginx -t`, reload. Until then the application guard runs open and warns at startup,
which is the safe direction — and the poller settles anything a blocked or
dropped delivery would have missed.

#### Done — PR 11: margins in basis points (migration 097)

Money columns were already `BIGINT` rupiah and exact. The **multiplier** was
not: margins were `double precision`, and each order's split was
`int64(float64(total) * pct)`. `0.70` has no exact binary representation, so
the product lands a hair under the true value and truncation drops a rupiah.

**Measured before changing anything**: across 25,070 price/margin combinations,
120 came out one rupiah short — about 1 in 200, always downward, and mostly out
of the operator's 70% share. Small per transaction, but systematic and
permanent.

Margins are now integer basis points (`1500` = 15.00%), and the split is
`total * bps / 10000` — exact integer arithmetic end to end. The sum constraint
became exact too: the float version needed an epsilon (`> 1.0 + 1e-9`) to avoid
rejecting a split that added up to exactly 100%.

`699110 × 70%` now returns 489,377 where the float version returned 489,376 —
that exact case is pinned in `order_split_test.go`, along with a property test
that the parts never exceed what was charged across a range of prices,
quantities and margin combinations.

The API contract changed with it: the old `double *_margin_pct` fields are
`reserved` and replaced by `int32 *_margin_bps`. The dashboard form still works
in whole percent, because that is how operators think, and converts at the
boundary. Nothing else in the web app referenced margins at all.

#### Done — PR 12: resolving a held transaction

`HELD` was a dead end: only someone with database access could clear it.
`ResolveHeldOrder` gives it two exits, from the dashboard's Tinjau action.

- **Terima** — the operator attests the difference was settled another way
  (cash at the counter, a top-up, or a shortfall they chose to waive). The
  order becomes `PAID`, so the commission settles and becomes payable. Nothing
  outside the system confirms that attestation, which is why it is always
  audit-logged with who decided it and the exact discrepancy accepted.
- **Tolak** — the order becomes `FAILED` and the commission recognised at
  placement is reversed. The money goes back outside the system, like every
  other refund here.

Only a `HELD` order moves, enforced in the UPDATE itself, so a repeated click
resolves nothing twice. `held_reason` is kept rather than cleared: why it was
held is part of the record.

`paid_amount_idr` and `held_reason` are now on the `Order` message, so the
dialog can show the bill against what actually arrived and name the shortfall.

Verified: accepting settles and makes the commission payable; rejecting closes
it and returns the commission to zero; and a resolved transaction cannot be
resolved a second time.

#### Done — PR 13: supplier cost and the price floor (migration 098)

Owner: *"harus selalu ada validasi harga modal dari supplier ... baik itu
nantinya di isi manual di pengaturan routing product, atau ... otomatis selalu
terupdate jika transaksi pernah terjadi pertama kali."*

Nothing recorded what a product costs to supply, so nothing could tell that a
price was below it. Selling at a loss looked exactly like selling at a profit,
and the margin split came out of a number with no floor underneath it.

**Two ways to know the cost, because they carry different weight.**
`supplier_cost_source` records which:

- `MANUAL` — entered by hand in the product's settings.
- `OBSERVED` — read back from the supplier's own response when a fulfilment
  actually happens, and refreshed on every later one, so a supplier raising
  their rate is noticed rather than silently absorbed.

**An observed cost cannot be overwritten by a manual one.** What a supplier
actually charged outranks what somebody typed, and letting a stale manual entry
replace it would defeat the point of observing costs at all.

`supplier_cost_observations` keeps the history behind the current figure —
append-only, like every other money record here — so a cost that moves can be
seen moving. One observation per order, so a retried fulfilment reports the
same purchase rather than inventing a second one. The observation and the
promotion happen in one transaction: a product whose cost disagreed with its
own latest observation would be worse than one with no cost, because it would
look authoritative.

**The floor is checked at the moment of sale**, on all three order lanes — not
only when the price was set. An observed cost moves on its own, so a price that
was fine last week can be underwater today with nobody having touched it.

**A product with no known cost sells without the check.** That is the honest
position: refusing would block every product nobody has costed, which today is
all of them, and inventing a floor would be worse than admitting there is none.

**Not wired yet:** nothing calls `RecordObservation`. It gets called from the
supplier fulfilment path, which does not exist — see the digital catalogue note
above. The manual side has no UI yet either.

#### Done — PR 14: TawafiqHub platform admin panel (migration 099)

Owner: *"harus ada 1 side lagi untuk ADMIN Tawafiqhub, agar ... setting yang
bisa dari web-based, bisa dilakukan dari web tidak harus lewat terminal."*

TawafiqHub had no identity in this system. Every user belongs to an operator,
and operator staff are confined to their own tenant — correctly, but it left
platform work with nowhere to happen except a terminal on the VPS.

**Access is a row, not a flag.** `platform_admins` is keyed by Better Auth user
id, with no self-service path to it. Granting is a deliberate INSERT by someone
with database access — which is the point: the panel it unlocks is what removes
the need for database access for everything else.

Not a column on Better Auth's `"user"`: Better Auth owns and migrates that
table, and platform access is the widest privilege here, so it should be
something somebody has to add on purpose rather than a boolean any code path
touching a user could flip.

**The authorisation check is the security-critical part**, and it is deliberately
visible at the top of each method rather than hidden in an interceptor. Nothing
in `PlatformService` is tenant-scoped — that is what makes it the platform's
view — so `requirePlatformAdmin` is the only thing between a signed-in operator
user and every other tenant's data. It reads the table on every request: a
revocation bites on the next call, which is worth far more than a cache.

`AmIPlatformAdmin` is the exception, callable by any signed-in user, because it
answers only about the caller — the web app needs it to decide whether to show
the panel without first calling something it may not be allowed to call.

Verified over real HTTP, through the real interceptor: no session →
`unauthenticated`; a **genuine operator owner's session** → `permission_denied`;
granted → works; revoked → refused again on the very next request.

**`/admin` gives two things today:**

- **Travel** — every tenant with plan, subscription status, pilgrim and product
  counts, and the number of transactions sitting in `HELD`. That last one is
  otherwise invisible unless an operator happens to look at their own orders.
- **Harga Modal** — the manual side of PR 13, which had no UI. Products with no
  supplier cost are listed first, since those sell with no price floor beneath
  them. A cost observed from a supplier cannot be overwritten here, and the
  panel says so rather than failing opaquely.

**To grant the first admin** (there is no other way, by design):
`INSERT INTO platform_admins (user_id, note) VALUES ('<better-auth-user-id>', 'founder');`

#### Done — PR 15: two-factor authentication, and the admin panel behind it

Owner: *"panel ADMIN ini hanya boleh di akses oleh saya saja, dan ada validasi
2fa juga untuk saya sendiri."*

**Better Auth's own `twoFactor` plugin**, not a hand-rolled TOTP. Rolling our
own would mean owning secret storage, clock skew, replay windows and recovery
codes — all of which the plugin handles and all of which are easy to get subtly
wrong. `npx auth@1.6.28 migrate` adds the `twoFactor` table and
`user.twoFactorEnabled`.

**The Go API needed no change at all**, and that is worth understanding rather
than assuming: the plugin does not create a session until the code verifies —
the pending one is discarded. So a session row existing in the database already
means the second factor was passed, and the API validates sessions by looking
them up. Verified against Better Auth's documentation rather than inferred.

**Platform access now requires an enrolled second factor.** Being in
`platform_admins` is no longer sufficient. Without this the second factor would
be optional for precisely the identity that can read every tenant's data.
Reported distinctly from a refusal (`failed_precondition`, and a separate field
on `AmIPlatformAdmin`) because the remedy differs: enrol, rather than ask for
access. Verified over HTTP: granted-but-unenrolled is refused, enrolled opens.

**Enrolment lives at `/keamanan`, outside the admin gate** — deliberately. If
enrolling could only be reached from behind the gate that requires it, the first
admin could never get in. It asks for the account password first (proof of
ownership, not just possession of an unlocked laptop), then verifies a code
before switching on, so nobody can lock themselves out with an authenticator
that was never set up correctly.

**Sign-in handles the challenge in place**, in `AuthForm`, rather than
navigating away — a mistyped code should not cost the user the password they
just typed. Google sign-in is untouched: those accounts already carry whatever
second factor the Google account has.

**`/admin` now returns not-found for accounts without access.** Honest framing:
this hides the panel, it does not protect it. The bundle is downloadable by
anyone signed in, so the real control remains the server refusing every
`PlatformService` call — which is separately tested against a genuine operator
owner's session.

**Recovery codes are shown once, at enrolment.** They are the only way back
from a lost phone; without them, recovery needs database access, which is the
thing the panel exists to remove.

**Not done:** 2FA is optional for everyone except platform admins. Whether staff
or jamaah should be required to enrol is still a decision — `AuthForm` is the
single sign-in surface for every role, so requiring it there would put an
authenticator app between a jamaah and their schedule.

#### Owner's decisions, 2026-08-27 — read before touching any of the below

These are rulings, not preferences. Several of them invalidate assumptions
baked into code that already exists, so they are recorded verbatim in intent.

1. **An agent buying on their own account earns no commission at all** — not to
   themselves, and not to the agent above them. Reason given: agents already
   buy at a special cost price from TawafiqHub, so a commission on top would be
   paying twice. This *simplifies* the agent-as-buyer work: no referral lookup
   on that path, commission is flatly zero.

2. **Digital products are managed by TawafiqHub, but each travel sets its own
   markup** on top of TawafiqHub's cost — separately for their agents and for
   their jamaah. So price is layered: platform cost → travel markup → agent
   price → jamaah price.

   **Referral depth: one level, revised the same day.** The owner originally
   asked for a nine-level tree, then cut it to one after seeing what nine would
   cost. Sideways stays unlimited.

   That is a significant piece of luck: one level is exactly the shape already
   built. `orders.agent_id` names the single referrer, one commission entry is
   written, and every "commission for this order" read assumes one earner —
   all of which stays correct. **The layered pricing is still to build; the
   referral chain is not.**

   If depth ever grows beyond one, the ledger itself copes (append-only entries
   keyed per agent), but attribution, the split calculation and those reads all
   assume a single earner and would each need reworking. Worth knowing before
   anyone agrees to it casually.

3. **Two-factor applies to travel staff and jamaah too**, not only platform
   admins, because both will be transacting. Currently only platform access
   requires it (PR 15). Note the consequence already flagged: `AuthForm` is the
   single sign-in surface for every role, so this puts an authenticator app
   between a jamaah and their schedule. Owner has decided that trade is worth
   it.

4. **The admin side needs a real management architecture**, not the two tabs it
   has now: product management, price management, product routing, supplier
   response and callback handling (accept, parse by regex, update status from
   it), log management, user/account management, and transaction management
   (monitoring and operations).

5. **Receipts must be previewable by every user who transacted**, from their own
   transaction history, and printable or saveable as PDF. Not operator-only.

6. **Least-privilege database role** — done, see PR 16 below.

7. **Browser rendering** — done, see PR 17 below.

#### Done — PR 16: least-privilege database role (migration 100)

The app connected as the database superuser, which made the append-only
guarantee weaker than it looked: a superuser can disable the triggers with one
statement, or drop the tables. The rule held only for code that played along.

`safrat_app` holds `SELECT, INSERT` on the four money tables
(`agent_commission_entries`, `pilgrim_balance_entries`, `order_refunds`,
`supplier_cost_observations`) and full DML everywhere else.

**Measured against the real schema, not assumed.** As that role: `UPDATE` on a
ledger refused; `DELETE` refused; `ALTER TABLE ... DISABLE TRIGGER` refused
(not the owner); `DROP TABLE` refused; and **`ON DELETE CASCADE` from a parent
still works**.

That last result is the one worth understanding. Referential actions bypass
privilege checks, so cascades keep working and tenant teardown is unaffected —
but it also means privileges alone would allow ledger rows to be removed
indirectly by deleting an operator. The append-only trigger stops that, and
this role cannot switch it off. **The two controls cover each other's gap;
neither is sufficient alone**, which is why both are kept.

New tables inherit full DML by default, deliberately: a forgotten grant would
take the application down, while a forgotten revoke only weakens a guarantee.
DEPLOY.md §12b carries an audit query to catch money tables that forgot theirs.

**Cutover is manual and not yet done.** The role is created `NOLOGIN` so no
password is ever in git. Steps, verification and a one-line rollback are in
DEPLOY.md §12b. Migrations must keep running as `safrat` — they need to own and
alter tables, which is exactly the power the app is giving up.

#### Done — PR 17: the money screens, opened in a browser at last

`e2e/money-screens.spec.ts` renders the orders dashboard, both of its dialogs,
two-factor enrolment and all three states of the platform panel, writing a
screenshot of each. Assertions are shallow on purpose — the point was to look
at them, and the things that go wrong here are invisible to every other kind of
test in this repo.

**Three defects, none of which any existing test could have caught:**

1. **Money formatted without separators in every string Go builds.** A held
   transaction read *"Nominal dibayar Rp4200000 tidak sama dengan tagihan
   Rp4500000"* on a screen where everything else said `Rp4.500.000`. These
   strings are read by people — audit entries, hold reasons, errors shown
   straight to an operator. A `rupiah()` helper now formats them, applied to all
   eleven call sites, with the negative case (`-Rp1.000`, sign outside the
   currency) covered since reversals are stored negative.

2. **The platform operator list had no limit.** It rendered every tenant in the
   database on one page — several hundred rows locally, and unusable for a
   platform with real tenants. Now bounded at 100 and ordered by held
   transactions first, so what actually needs attention is at the top rather
   than buried by creation date. The UI says when the list is truncated, since
   an admin should not conclude a tenant is missing because it fell off the end.

3. **The transport test harness had been leaking operators since forever.**
   `seat_assignments.operator_id` has no `ON DELETE CASCADE` — a known quirk,
   documented in CLAUDE.md and worked around in the seed file — and the
   harness's cleanup discarded the resulting error. Every run that assigned a
   seat left its whole operator behind: 84 operators and 315 pilgrims had piled
   up. Not caused by the ledger triggers, which is what I checked first. The
   cleanup now removes seat assignments first and logs anything that still
   fails; the accumulated rows were verified to hold no orders and no money
   entries before being cleared.

Screenshots land in `e2e/.screens/` and are gitignored — evidence for whoever
ran it, not something to carry in the repository.

**All rendered now** — see PR 25.

#### Done — PR 18: supplier routing and its management surface

Owner asked for a real management architecture on the admin side: products,
prices, product routing, supplier response and callback handling (accept, parse
by regex, update status from it), logs, users, and transactions. Routing is the
piece everything else hangs off, so it went first.

**Schema (migration 101), all platform-owned — no `operator_id` anywhere.** A
travel does not get to point a product at a different supplier, or see another
travel's routing.

- `suppliers` — name, stable `code` used in logs so renaming for display never
  orphans history, and `credential_env_var` naming *where* the worker reads
  credentials from. **Credentials are never stored in the row**: a database dump
  carries no secrets, and rotating a key touches no data.
- `product_routes` — one route per product, enforced by a unique constraint. Two
  active routes would make "which supplier did this sale go to" unanswerable
  after the fact.
- `supplier_response_rules` — ordered patterns, editable from the panel.
- `supplier_request_logs` — every exchange, append-only, with the parsed fields
  stored *alongside* the raw body rather than instead of it, so a rule that
  turns out to be wrong can be re-read against what actually arrived. Also
  revoked from the app role's UPDATE/DELETE, like the money tables.

**Why parsing is data and not code.** Suppliers in this market answer in JSON,
form bodies, or plain SMS-style text, and change shape without warning.
Hard-coding a parser per supplier means a deploy every time one shifts a field.
A regex can only read, so a bad rule produces a wrong status — never arbitrary
execution inside the worker.

**`internal/supplier` reads a response into an outcome.** Rules apply in
priority order, first match decides, and named capture groups lift out the
supplier's reference and what they charged. Tested against the shapes that
actually occur: JSON, the same JSON a month later with quoting dropped, and
plain text.

Three decisions in there worth keeping:

- **`UNMATCHED` is its own outcome, not a failure.** A response nobody taught
  the system to read is a gap in the rules. Folding it into FAILED would refund
  transactions the supplier may well have delivered.
- **A rule that does not compile is skipped, not fatal** — but the skip is
  *reported*, not swallowed. One bad pattern must not stop a later correct one
  from recognising a delivered transaction; equally, coverage quietly
  disappearing is how a supplier drifts into producing nothing but UNMATCHED
  with nobody realising the rules stopped working rather than the supplier
  changing shape. `Reading.SkippedRules` carries them, and the rule tester
  shows them.
- **An unstated cost stays nil, never zero**, and an amount that cannot be read
  confidently is refused rather than guessed. A cost misread by a factor of a
  hundred would set a price floor that either blocks every sale or protects
  nothing.

Rules are validated when **saved**, so a bad pattern is refused in the panel
rather than discovered over live transactions at three in the morning.

**Management surface, all behind the platform gate.** `PlatformService` gained
nine methods: supplier list and upsert, route list and save, rule list, create
and deactivate, a rule tester, and the log view.

Three choices in there worth keeping:

- **Suppliers upsert on `code`, not id.** Saving an existing code updates it,
  so renaming a supplier for display can never create a second one alongside
  its own history. Verified over HTTP.
- **Rules are deactivated, never edited.** Changing a pattern in place would
  silently change how past logs *should* have been read; a new rule plus
  deactivating the old one keeps that history legible.
- **`TestResponseRules` runs a sample through the live rules, touching
  nothing.** Writing a pattern blind against real money is how bad rules reach
  production. The panel can try one first, and the tester uses exactly the code
  the worker will.

Rules are compiled at save time, so a malformed pattern — or one naming a
capture group it never defines — is refused with `invalid_argument` rather than
discovered over live transactions. Both cases are covered by the HTTP test,
along with an operator owner being refused the whole catalogue.

**Still to build:** the admin screens for all of this (the RPCs exist, the UI
does not); the callback endpoint feeding responses through the reader; the
fulfilment worker that calls suppliers and records
`supplier_cost_observations`; and transaction monitoring.

#### Open — the residue of the three parsing decisions

The owner asked whether those three choices leave gaps. Two of them do, and one
is now closed:

- **Closed: a skipped rule is no longer silent.** `Read` reports what it could
  not apply and why, and the tester surfaces it.
- **Closed: an UNMATCHED transaction is now watched.** `worker/fulfilment.go`
  sweeps every 10 minutes for anything NEEDS_REVIEW, or SENT and silent for over
  an hour, and logs each one individually at WARN with the jamaah, product and
  supplier named. "Seven are stuck" is not actionable; "this jamaah's pulsa
  never arrived" is. The original text of the gap follows, for the reasoning:
- ~~**Open: an UNMATCHED transaction hangs with nothing watching it.**~~ Refusing to
  treat an unreadable response as failure is right — refunding a transaction the
  supplier may well have delivered is worse and irreversible — but the
  consequence is that it waits for a human. There is an index and a filtered log
  view, and **no alert of any kind**. If nobody opens the panel, a jamaah's
  transaction waits forever. This needs a sweep with a threshold and a
  notification, in the same shape as the SOS escalation worker.
- **Not a gap: an unstated supplier cost stays nil.** Storing zero would set a
  floor of Rp0 that passes everything, *and* mark the cost `OBSERVED`, which
  cannot be overwritten manually — a false zero that locks itself in. Nil puts
  the product in the panel's "no cost recorded" queue instead, where it is
  visible and fixable.

#### Done — PR 19: fulfilment, receipts, and two-factor for everyone

Four pieces, built together because they interlock.

**Fulfilment exists at all now** (migration 102). A paid order for a digital
product used to end there: money taken, nothing ever sent, no state saying so.
`order_fulfilments` is deliberately separate from `orders.status` — "did they
pay" and "did it arrive" are different questions, and one column carrying both
makes *paid but undelivered* inexpressible, which is exactly the state that has
to be visible.

- One fulfilment per order, enforced by a unique constraint rather than by the
  worker checking first: two workers picking up the same paid order both pass a
  check-then-act, and the result is a jamaah's pulsa sent twice at our cost.
- Claiming is a conditional UPDATE, not read-then-write. The transition *is* the
  lock; a worker that reads PENDING and then writes SENT can be overtaken
  between the two statements.
- A product with no active route still opens a fulfilment, immediately flagged
  NEEDS_REVIEW. The jamaah has paid either way, and a paid order with no record
  of owing anything is precisely how a transaction disappears.

**The callback endpoint** (`POST /webhooks/supplier/{token}`) reads what a
supplier posts with that supplier's own rules. Plain net/http, not Connect —
suppliers post whatever shape they like and would not speak Connect if asked.
It answers 200 whatever it makes of the body: a supplier that gets an error
retries, and a retry cannot fix a reference we do not recognise or a rule
nobody has written. The record is in the logs either way.

Verified end to end: a delivered transaction settles and **learns the supplier's
price from the same message**, giving the product a floor it did not have; six
concurrent callbacks produce one delivery and one cost observation; an
unreadable response becomes NEEDS_REVIEW and appears in the attention queue; a
human can close it and **a later supplier message cannot overwrite that
decision**; and an unknown token settles nothing.

**The stuck sweep closes the gap** the parsing decisions left. Holding rather
than refunding is only defensible if somebody is actually told to look.

**Receipts** (migration 103). Every transaction now has a number a person can
quote — `INV-YYYYMM-NNNNNN`, from one global sequence. A per-tenant counter
would have to be read and incremented, which two concurrent checkouts race on,
and the usual fix serialises every sale in the tenant. Numbers are therefore
not contiguous within one travel, which nobody asked for and which would leak
how many sales other tenants made. `TransactionReceipt` renders one from the
jamaah's own history, printable — and PDF is the browser's own print dialog
rather than a server-side renderer, which would mean a second layout to keep in
step with this one.

**Two-factor for staff and jamaah**, with a difference that is a judgement call
worth reading: staff are **blocked** until enrolled; jamaah get a **prompt they
can dismiss**. Behind the pilgrim shell sits SOS. Standing between a jamaah in
distress and the button that summons help, because they have not installed an
authenticator app, is a hazard rather than a security improvement. The owner's
instruction was that jamaah use two-factor too, and they will — the judgement
is only about whether the app refuses to open in the meantime.

**Two things the browser found**, neither of which any other test could:

1. **Enabling 2FA silently broke the e2e setup.** Better Auth issues no session
   at all when an account has TOTP enrolled — sign-in returns a challenge — so
   a flag left enabled by an earlier run made the setup save an *empty* storage
   state, and every later spec failed with something that looked nothing like
   the cause. The setup now clears it for fixture accounts, which cannot answer
   a challenge anyway.
2. **A failed run left its platform grant behind**, so the next run started with
   access it was supposed to be testing the absence of. The test now clears it
   first rather than only afterwards.

**Admin screens** for suppliers and their reading rules, including the rule
tester — try a pattern against a real sample before trusting it with money.

**The outbound call is built too** — see PR 20 below.

#### Done — PR 20: calling suppliers (migration 104)

Owner: *"rata rata API, ada juga yang http get biasa, ada yang XML RPC ...
standard host to host."*

**The request is configuration, exactly like the response rules.** These shapes
do not converge and there is no prospect of them doing so, so a client written
per supplier would mean a deploy every time one is added. `internal/supplier`
builds all four from a row: `REST_JSON`, `HTTP_GET` (everything in the query
string), `FORM_POST`, and `XML_RPC` for the older host-to-host terminals.

**Signatures are recipes**, `md5:{{username}}{{credential}}{{reference}}` and
the like, with md5/sha1/sha256. md5 and sha1 are weak and are nonetheless what
a good number of these providers mandate — supported because refusing them
means refusing the supplier, not because either is sound.

**Credentials never touch the database.** The row names the environment
variables; the values are read at send time. A dump carries no secrets and
rotating a key touches no row.

**The log copy is redacted separately**, not by finding secrets in the finished
request and blanking them — that would depend on a credential being distinctive
enough to find, which it is not. The template is substituted twice, once for
real and once with the secrets replaced.

**SSRF is refused before anything is sent.** Supplier addresses are typed in by
a person, and one pointing at loopback or a private range turns this worker into
a request forwarder aimed at whatever the server can reach — cloud metadata
endpoints being why this matters rather than a theoretical concern.

That guard refused the tests too, since every stub server binds to 127.0.0.1.
Loopback is therefore permitted **only when `testing.Testing()` is true** — a
condition that cannot hold in a compiled server and that nobody can set by
accident. Private and link-local stay refused even under test, because those are
the addresses that actually matter.

**Dispatch is immediate, with a sweep underneath — corrected after the owner
pointed out that a digital product's SLA is seconds, not a minute.** The first
version made the sweep the mechanism, which was wrong: a jamaah buying pulsa
expects it now. Sending is enqueued the moment payment settles and picked up in
milliseconds; the sweep every minute is the net for an enqueue that failed, a
Redis restart that dropped the queue, or a worker that died mid-send.

Neither is sufficient alone — the queue is fast but losable, the sweep durable
but slow — so the common case is immediate and the uncommon case is merely
late. A failed enqueue never fails the payment: the money has settled and the
row already records the debt. Verified that both paths racing the same order
call the supplier exactly once.

**Nothing that fails becomes FAILED.** A transport error, an unresolvable
address, a bad recipe — none of them prove the supplier did not deliver, so all
of them go to a person. The attempt is still counted, so a request that was sent
and lost never looks like one that was never sent.

Verified: a GET terminal receives our order id as the reference and the SKU,
settles DELIVERED, learns the price from the same answer, and **is not called a
second time** on the next pass; a supplier that never answers becomes
NEEDS_REVIEW with the attempt counted; and an address pointing at cloud metadata
is refused with a reason that names it.

#### Done — PR 21: payment window, polling backoff, and platform transaction monitoring

Owner: *"semua transaksi harus tertulis ... once dia belum bayar, atau sudah
bayar pun"*, *"limit dari create transaction sampe ke payment itu gaboleh
terlalu sebentar"*, and *"cepat dan aman, tercatat, dan tidak ada celah
kerugian."*

**Every transaction was already written before payment** — orders are created
`PENDING`, then settled — so that requirement held. What was missing was
anywhere on the platform side to *see* them, which the new Transaksi tab
supplies: every transaction across every tenant, paid or not, with **two status
columns** because "was it paid" and "did it arrive" are different questions and
one column carrying both hides the state that matters.

**The payment window is now explicit.** It was never set, so it silently used
Xendit's default. Twenty-four hours is generous on purpose: scanning a QRIS
code or carrying a virtual account number to a bank is not instant, and
somebody buying at night may finish in the morning. An unpaid invoice holds no
stock and blocks nothing, so a long window costs nothing.

**But a day-long window created a real problem**, which is the interesting part
of this change. The poller guards against a lost webhook by asking the gateway
every two minutes — for a whole day, per abandoned checkout, across every
tenant. Roughly seven hundred calls each. Exhausting the gateway's rate limit
would stop settlement working for *everybody*, which is a far worse loss than
the dropped webhook the poller exists to catch.

So checks back off with age: every 2 minutes for the first quarter hour, every
10 minutes up to two hours, hourly after that. Payment usually happens in the
first minutes, so that is where the attention goes. A check is recorded whether
or not the gateway answered — otherwise one unreachable invoice would be
retried on every pass while everything behind it starved.

#### Race conditions and disaster cases — audited 2026-08-27

Asked directly whether the money paths are safe from races. Honest answer: the
ones below are, each with a concurrency test behind it rather than an argument.
Two were **not**, and were closed during the audit.

**Protected, and tested under concurrency:**

| Hazard | What stops it |
| --- | --- |
| Paying one order twice | `PENDING → PAID` conditional UPDATE; a redelivered webhook moves nothing |
| Refunding twice | one refund per order + idempotency index + `FOR UPDATE` on the order |
| Crediting commission twice | unique `(order_id, kind)`, `ON CONFLICT DO NOTHING` |
| Two orders from one double-click | unique `(operator_id, idempotency_key)` |
| Sending a supplier the same order twice | the claim *is* a conditional UPDATE, not read-then-write |
| Learning a cost twice from a retried callback | unique on `order_id` |
| Withdrawing more commission than earned | advisory lock per agent |
| Editing a ledger entry | append-only trigger, plus the least-privilege role |

**Closed during this audit:**

1. **Refunding something the supplier had already delivered.** `RefundOrder`
   checked the payment status and nothing else, so refunding a delivered pulsa
   meant paying the supplier *and* giving the money back — a direct loss with
   nothing to stop it. Now refused, and refused while a delivery is still in
   flight too, since an answer that has not arrived is not the same as a
   failure.

   Writing the test exposed a hole in that fix: I refused refunds on delivered
   goods and pointed at a door that did not open — `DELIVERED` could not be
   corrected at all, which would trap an operator holding a jamaah's money with
   no lawful way to return it. Correcting a delivery is now permitted, requires
   a reason, and records who decided. Somebody could flip a real delivery to
   failed in order to refund it; the act is allowed but never anonymous.

2. **A paid order owing a delivery that nothing recorded.** Marking an order
   paid and opening its fulfilment are two writes, not one transaction. A
   process dying between them left a paid order that *no dispatch path could
   ever see*, because every one of them starts from the fulfilment row — the
   jamaah had paid and no part of the system believed anything was owed. The
   sweep now recovers these, set-based and keyed by the unique constraint so it
   cannot race the normal path into a second row.

**Known and accepted, not closed:**

- Side effects after payment (commission, fulfilment) are not in the settling
  transaction. Making them so would mean holding a database transaction across
  an outbound call. Both are instead reconciled by sweeps, which is the same
  trade the rest of the system makes.
- A supplier that charges us and never answers, ever, stays NEEDS_REVIEW until
  a person decides. That is deliberate: no automatic rule can tell that case
  apart from one where delivery genuinely happened.

#### Done — PR 22: KYC encrypted and made a record of its own (migrations 106)

Owner asked for encryption at rest for KYC, then added that **the data must be
independent with a clear relation to the account, so it can be produced and
checked on request.** The second requirement changed the shape, for the better.

**KYC is its own record now.** It used to be columns scattered across `agents`
and `pilgrims`: an identity was a property of whichever role record happened to
hold it, and "whose identity is this" had no single answer. `kyc_records` names
the account (`user_id`) and the role record it was collected against, so it can
be produced by following one relation rather than reconstructed by guessing
which table the person turned up in. A person holding two — an agent who is
also a registered jamaah — keeps both, since collapsing them would hide exactly
the connection somebody asking is trying to see.

**Encrypted before it reaches a row.** AES-256-GCM, random nonce per value, key
from `KYC_ENCRYPTION_KEY`. The same identity number never produces the same
ciphertext, so equal values reveal nothing — which rules out searching by them,
and nothing does. GCM also authenticates: a tampered value fails to open rather
than decrypting to something plausible, and a wrong key fails loudly rather
than silently corrupting every record it touches.

**Without a key, KYC writes are refused rather than stored in the clear.** A
fallback to plaintext because a variable was missing is the exact failure this
prevents, and it would look like success until a breach made it obvious. The
rest of the application still starts and works.

**Legacy plaintext is moved by a Go task, not a SQL backfill.** The key lives in
the process, not the database, so SQL could only copy plaintext into the new
table and leave it there "until something came along" — which is how
unencrypted identity numbers survive for years. Each record is moved and its old
column cleared in one transaction, so no row ever holds it twice or not at all.
Scheduled hourly rather than run once, because "run this after deploying" is an
instruction somebody eventually does not follow.

**Operational warning, in `.env.example` too:** losing the key means losing
every number encrypted with it — there is no recovery path, by design. It must
be backed up *separately from the database backups*, or the two together defeat
the purpose.

Verified: what is submitted is not readable in the table, reads round-trip,
lookup by account works, resubmitting clears a prior verification (a
verification applies to what was checked, not to what replaced it), and the
legacy move leaves nothing behind and does nothing on a second pass.

#### Done — a customer service route

`CustomerServiceButton` on the jamaah's transaction history and product pages,
opening WhatsApp to 0812-8303-1003 with the receipt number already in the
message. Placed where money moves rather than in a settings page: the moment
somebody needs it is the moment a payment failed, and asking them to go looking
then is asking them to give up.

#### Owner's ruling — transaction records are never reduced (2026-08-27)

> *"Transaksi memang akan selalu mencatat nomor tujuan, tidak boleh data
> transaksi hilang sama sekali."*

This settles the conflict flagged earlier between append-only logs and the
right to erasure. The destination number stays in the transaction record, and
nothing is deleted.

That position has a basis, not just a preference: UU PDP allows retention where
another legal obligation requires it, and a transaction record is exactly that.
It is also the only thing that makes a dispute arguable — a jamaah claiming
their pulsa never arrived, or a supplier claiming it did, both need the number
it was sent to.

What the ruling does **not** remove is the obligation to hold it safely. The
identity numbers are encrypted; the destination number in a transaction is not,
because it is part of the record itself. Worth revisiting if a regulator ever
asks how the two are told apart.

Separate backup database: owner will do it later.

#### Done — PR 23: a product catalogue with the numbers a catalogue needs (migration 107)

A product had a name and a price. Three things were missing, and conflating
them is how a digital catalogue goes wrong:

- **`code`** — what a person quotes. `PULSA-TSEL-10K`, not a UUID. Unique per
  operator, because two travels may both sell `PULSA-10K` and neither should
  need to know the other exists. Left blank, it is derived from the name, so
  nobody is forced to invent one and nothing slips past the uniqueness index.
- **`nominal_idr`** — the face value the customer receives. Pulsa of 10,000
  sold for 11,500. **Nullable, not zero**: a travel package has no face value;
  it does not have one worth nothing.
- **`supplier_cost_idr`** — already there from PR 13, what we pay.

Four distinct numbers where only one existed. Without nominal, "10,000 pulsa
sold for 11,500" and "11,500 pulsa sold at cost" are the same row, and nobody
can tell a margin from a mistake.

Existing products were backfilled with a code derived from their name plus a
fragment of their id — ugly and unambiguous, and an operator can rename it.

**A flaky test of my own, caught and fixed:** the encryption round-trip asserted
that the ciphertext does not contain the plaintext, using a one-character input.
A single character turns up in random base64 often enough to fail on its own.
Short values are now round-tripped without that check, and the property that
actually matters is asserted against the decoded bytes instead.

#### Design reference — SUPERSEDED, and the Cashplus note was wrong (2026-08-30)

> The owner replaced the Cashplus.id direction outright: *"reference nya bukan
> cashplus lagi sekarang."* The previous entry described a white sidebar, vivid
> blue accent and breadcrumbs. **None of that applies.** It is removed rather
> than left with a note, because a stale reference reads exactly like a current
> one to whoever opens this next.

Current references live in the repository root:

- `landing-page-refference1.jpeg` … `4.jpeg` — SafarGo. Near-black hero with an
  Islamic geometric pattern, **warm orange as the accent**, heavy display type
  with one word picked out in orange, 3D Kaaba render, floating pill badges
  around the hero, then a white body: certification logo strip, stat cards with
  one filled orange, numbered `01`–`05` service list, dark testimonial card,
  dark footer.
- `PWA-and-Mobile-UI_refference.jpeg` — deep green header, white rounded cards
  on a light ground, circular icon tiles, a dotted timeline, and a bottom sheet
  with rounded top corners.

Two different languages on purpose: the marketing site is dramatic, the app is
calm.

**Where the landing page actually stands** (read from the rendered page, not
from the code): its *structure* already matches the reference — dark hero,
light body, bento product grid, one highlighted pricing card, FAQ accordion,
dark CTA and footer. What differs is the treatment:

1. accent is green, reference is orange;
2. hero uses a photograph, reference uses a 3D render with a pattern and
   floating pill badges;
3. three sections are missing — the certification strip (for a PPIU this is
   credibility, not decoration), the numbered `01`–`05` list, and the
   testimonial;
4. eyebrows are plain capitals, reference uses small pill badges.

So this is roughly a third of a job, not a rewrite.

**Undecided, and it changes what can be built:** photography or 3D. The current
page uses real photographs of jamaah, which is arguably stronger for an
Indonesian PPIU than a render. Matching the reference exactly needs 3D assets
that do not exist in the repository.

**Division of work while both agents are running:** design belongs to Codex.
Anything under `apps/web/components/landing/` and the `.landing-*` block of
`globals.css` is theirs. Feature work stays out of both.


#### Done — PR 24: account management, identity access, and an audit trail that was not there

**The bug worth reading first.** Every platform-level audit entry had been
failing silently since PR 14. `audit_logs.operator_id` is `NOT NULL` with a
foreign key, and a platform action belongs to no tenant — so granting platform
access, changing a supplier, reading an identity, all of it was discarded by an
error the caller ignored. **The code claimed an audit trail it did not have,
which is worse than having none.**

Found by a test asserting that reading somebody's NIK leaves a trace. Nothing
else would have noticed, because failure looked exactly like success.

Migration 108 makes `operator_id` nullable for platform actions and widens
`entity_id` from UUID to text — a Better Auth account id is not a UUID, and
granting somebody access is an action about exactly that. Encoding it into a
fake UUID to satisfy the column would have made the trail unreadable. The error
is now reported rather than discarded.

**Account management** removes the last thing that needed a SQL client:

- Every account across every tenant, with whether they hold platform access,
  whether they have a second factor, and how many live sessions.
- Grant and revoke platform access. **Revoking the last admin is refused** —
  it would lock the panel for everybody and the only way back would be the SQL
  client this replaces.
- **End every session for an account.** The response to a suspected takeover:
  resetting a password changes nothing for whoever already holds a live
  session, and nothing else in this system ends one early.

**Identity records are now readable, and reading is recorded.**

- The **list carries no identity numbers at all**. One careless screenshot of a
  list that did would leak everybody on it.
- `GetKycRecord` returns them in the clear and audit-logs every read, without
  exception. Reading somebody's NIK is not a neutral act, and the record of who
  looked is the only thing that makes the access reviewable.
- Rejecting requires a reason: a rejection nobody can act on gets resubmitted
  unchanged.

**The panel now has six tabs**, all rendered in a browser: Transaksi, Travel,
Harga Modal, Supplier, Akun, Identitas.

Search on the account list is deliberate rather than open paging — a platform
panel that will happily page through every account in every tenant is a data
export waiting for a curious employee.

#### Done — PR 25: every screen has now been opened in a browser

The standing gap is closed. `e2e/portal-screens.spec.ts` covers the three
surfaces the money-screens spec could not reach, because each needs a different
identity: a jamaah, a Muttawwif, and an agent.

The Muttawwif screen is worth calling out, because it is the visible proof of a
fix made days earlier. The commission ledger shows **both** the earning
(+Rp450.000) and its reversal (−Rp450.000), with the balance at zero — a
balance the list now *explains* rather than contradicts. Before the wallet was
moved off PAID orders, a refunded order's earning simply vanished from the list
while the balance dropped, and an agent had no way to tell what had happened.

The fixture makes the staff account additionally an agent who leads a group.
Both portals resolve identity from the signed-in user, so one account reaches
both, and an owner is never treated as a restricted member — so this widens
nothing.

Screens covered end to end now: orders dashboard and both its dialogs,
two-factor enrolment, all three states of the platform panel, its six tabs,
the jamaah's history and receipt, the customer service route, the Muttawwif
ledger, and the agent recap.

#### Done — PR 26: making the encryption key findable without making it exposed

Owner asked how to keep `KYC_ENCRYPTION_KEY` safe *and* easy to find again.
Those pull against each other, so the answer is split: storage advice in
DEPLOY.md §12c, and a mechanism that answers "is this the right key" without
anybody handling the key.

**A fingerprint** — eight hex characters of a SHA-256 over the key. It
identifies a key without revealing it; reversing it would mean breaking
SHA-256, and 32 random bytes hold far more entropy than it could leak. Safe in
a log, in a password manager note, or read out over the phone.

- **Printed at every startup**, alongside a count of records by the key that
  sealed them.
- **Stored on every sealed record** (migration 109), so the data itself says
  which key opens it.
- **A mismatch is reported loudly at startup** — "317 records were sealed with
  a1b2c3d4, this deployment loaded 99887766" — rather than being discovered one
  unreadable identity at a time, days later.

That makes a candidate key checkable *before* it is deployed: set it, start the
API, compare. Nothing is decrypted to find out.

It also makes rotation legible: two fingerprints appear while one runs, and the
old one's count reaching zero is what "finished" means. **Rotation is now built
too** — `cmd/rotatekyc`, a separate command rather than a scheduled task,
because rotation needs both keys in one process and a long-running server
holding both would be one configuration mistake away from writing new data with
the old key.

Resumable by construction: it only selects rows still stamped with the old key,
so it continues where it stopped and does nothing once finished. Value and stamp
move in a single UPDATE, so a row can never carry a fingerprint that does not
match the value beside it — which is the property the startup check and the
mismatch warning both depend on. A record the old key cannot open **stops** the
run rather than being skipped.

The order is documented and matters: rotate first, change the variable second,
and keep the old key until backups taken before the rotation are out of
retention — those still hold records only it can open.

**Storage guidance, stated plainly in `.env.example` and DEPLOY.md:** a
password manager, with a sealed offline copy as a second line, and **never on
the VPS beside the database backups** — whoever takes that machine would
otherwise get the data and the key that opens it in one go, which is the exact
outcome encrypting it was meant to prevent.

#### Done — agent/Muttawwif self-purchase (migration 112)

The schema foundation from migrations 110–111 now has a complete production
lane rather than an unused buyer column:

- `ListMyPurchaseCatalogue`, `CreateOrderForSelf`, and `ListMyOrders` are
  session-derived RPCs available to restricted agent/Muttawwif accounts. The
  client never supplies an agent id.
- The quote is the agent price: platform base + operator markup. Agent markup,
  referrer and commission are all zero, with the existing database constraints
  enforcing the money rule beneath the service.
- The provider destination is frozen on `orders.destination` at checkout and
  supplier dispatch now reads it from the transaction. It no longer looks up a
  mutable pilgrim phone at send time. Existing digital orders were backfilled.
- Order, cashflow, platform-admin and fulfilment reads use buyer-safe joins, so
  an order with `pilgrim_id = NULL` remains visible everywhere. The operator
  dashboard labels the row as Agent/Muttawwif and does not offer the
  pilgrim-balance refund action for it.
- `/agent` has a "Beli Produk" tab with the correctly quoted digital catalogue,
  destination input, Xendit checkout, retry-stable idempotency key and the
  buyer's own pending/paid history.

Verified on isolated PostgreSQL through migration 112: catalogue price,
nullable pilgrim/exact buyer, frozen destination, zero agent markup and
commission, operator list, platform transaction list, agent history, and paid
cashflow total. The pre-existing referral tests still pass alongside it.

#### Open — ordered

Items 1, 3, 4 and 5 of the original list are done (see PR sections above).
What remains, in the order I would take it:

1. **Digital product catalogue belongs to the platform, not to each operator.**
   Owner: *"clients travel umroh semuanya bukan penjual untuk product digital,
   yang punya jalur API ke supplier, dll hanya pihak tawafiqhub."* Today
   `products` are per-operator, so each travel invents its own
   `ROAMING_DATA`/`PPOB_CREDIT` rows and prices, and the platform has no
   catalogue at all. The foundation is now in place — supplier cost with a
   price floor, and an admin panel to manage it — but the ownership change
   itself has not started.

2. **Complete digital fulfilment and compensation.** Fulfilment records and
   supplier transport exist, but the remaining safety gates below still apply.
   - `EQUIPMENT` (physical): no delivery status, address, tracking or handover
     proof. Safe only while handover is manual and in person.
   - `ROAMING_DATA` (digital): no voucher or eSIM issued or stored.
   - `PPOB_CREDIT`: **no provider integration at all** — a jamaah pays and no
     credit is ever sent. Keep it disabled until it can be fulfilled.

   This is where `SupplierCostRepository.RecordObservation` finally gets called,
   and where a Saga-style compensating transaction genuinely earns its keep:
   charge → call supplier → supplier fails → refund. Idempotency is critical on
   the fulfilment side too, or one payment becomes two top-ups.

3. **Per-transaction receipts.** `/dashboard/pilgrims/[id]/invoice` is
   per-pilgrim and operator-only. Missing: a receipt per transaction with a
   referenceable number, and any way for the **paying account** to see or print
   its own proof. Owner asked for this explicitly.

4. ~~**Fraud and attempt limits.**~~ Done in migration 121. Gateway checkout is
   capped per buyer under a database advisory lock; velocity flags the fourth
   and fifth attempt for review, a matching payment on those orders enters
   `HELD`, and unresolved held money blocks a new checkout. See the 2026-08-29
   continuation near the end of this file.

5. **Database role without UPDATE/DELETE on the ledgers.** The application
   currently connects as a superuser, which can disable the append-only
   triggers outright — so today's "cannot be manipulated" guarantee is weaker
   than the triggers suggest. Operational work on the VPS, not code.

6. **`XENDIT_WEBHOOK_ALLOWED_IPS`** — needs a support request to Xendit, who do
   not publish the ranges. Config blocks are ready in nginx and Caddy.

7. **Browser rendering review.** Earlier jamaah/Muttawwif/agent transaction
   pages, the refund and held-order dialogs, `/admin`, and the new agent
   purchase tab pass static checks but still need a visual pass. Playwright
   exists in `apps/web/e2e/`.

8. **Caddy cutover** — still manual, still unstarted (`deploy/caddy/README.md`).

#### Standing rule for all of the above

Enforce uniqueness and balance checks **in the database**, never by
SELECT-then-INSERT: two concurrent requests both pass an application check.
Every externally visible operation needs an identifier stable across retries
but unique per attempt. See the fixes above for the patterns already in use —
partial unique indexes, advisory locks, `ON CONFLICT DO UPDATE`.

### Pre-release checklist — repository and image visibility

Deferred by the owner (2026-08-26) until closer to release. Recorded because the
first item is already live, not hypothetical.

**The container images are public right now.** Verified by pulling
`ghcr.io/agilalaydrus/safrat-api` with no credentials at all. They contain the
compiled Go binary and the Next.js bundle — the product itself. Anyone who knows
the name can download and run TawafiqHub on their own server. Making the *repo*
private does not change this: GHCR package visibility is separate.

**Making the packages private breaks deploys as written.** `deploy.yml` runs
`docker compose pull` on the VPS with no `docker login`, so it only works while
the packages are public. The failure would land *after* goose has migrated —
database ahead, containers behind, which is the worst state to recover from.

Before flipping either switch:
1. Add an ephemeral login to the deploy step — `docker login` → `pull` →
   `docker logout` — with a fine-grained, repo-scoped token from GitHub
   Secrets. Ephemeral on purpose: `docker login` writes credentials to
   `~/.docker/config.json` base64-encoded, not encrypted, so a permanent login
   leaves a reusable credential on disk for anyone with SSH.
2. Expect token expiry to break deploys silently months later; prefer failing in
   CI with a clear message over failing on the VPS mid-deploy.
3. Note that private repos consume Actions minutes (2,000/month on the free
   tier). Deploys run ~11 minutes each.

Also: the subscription bank account belongs in `.env.prod` on the VPS, never in
the repo — private or not. Private means "fewer people can see it", not "safe".

### Billing — operator subscriptions (2026-08-26)

Until this, `plan` was a label with nothing behind it: prices were published but
nothing could charge for them, and the only way to change a plan was UPDATE by
hand. Shipped in four commits: `2d113c5` schema, `a5f8e45` the gate, `ae9cae5`
the operator-facing screen, `35ba085` gateway settlement.

**Rules that matter**

- New operators get a **3-day trial**. Existing operators were seeded ACTIVE
  with 90 days — they are live and were never charged; a migration must not lock
  them out.
- Access is granted **by time (`access_until`), never by status**. A stale
  status can then never hand out free access.
- **Dashboard locks, everything else keeps running.** The gate sits after the
  staff session resolves, so the storefront, registration, and the pilgrim,
  leader and agent portals are untouched. Pressure lands on the operator who
  owes money, not on pilgrims who already registered.
- Billing procedures stay reachable while locked; everything else fails closed.
  A missing subscription row is *allowed* — locking a paying customer out over a
  missing row is the worse failure.
- The screen lives **outside the dashboard shell** because the shell loads gated
  data on mount and would not render for a locked operator.

**Bank transfer matching**

The unique amount suffix is the only thing tying a mutation to an invoice, so
uniqueness is enforced by a partial unique index over unpaid transfers — not by
checking before inserting, which would hand two simultaneous requests the same
amount. A test drives 40 concurrent issuances. `FindPayableByAmount` is written
and tested but **has no caller yet**: bank transfers are still settled by hand.

Before building a scraper, consider an aggregator with a real API (Moota,
Mutasibank). A scraper fails *silently* — the operator has paid, the system does
not know, and they stay locked out. The code shape is identical either way:
`FindPayableByAmount` then `MarkPaid`.

**Two traps found while building this**

1. The gate first landed in the `sessionOnlyProcedures` branch, which would have
   locked out **tour leaders** instead of operators. A compile error caught it.
   The anchor is now asserted unique so the slip cannot recur silently.
2. `err?.code === "unauthenticated"` in the dashboard shell **never worked**:
   Connect-ES exposes the code as a numeric enum, so the string comparison is
   always false. The new redirect uses `Code.FailedPrecondition`; the pre-existing
   string checks alongside it are still wrong and worth a separate fix.

**Do not rely on bank transfer in production until matching exists** — the
operator will pay and stay locked out. QRIS/card is safe: verified end to end
against the running API.

### Session log — 2026-08-26


**Pricing tiers map onto the white-label levels** (owner, 2026-08-26). The
`plan` enum from migration 001 already matches them:

| Plan | Tier | Entitlement |
| --- | --- | --- |
| `STARTER` | Starter PPIU | Platform subdomain. Landing page still fully customisable, and pilgrim / muttawwif / agent portals all included. |
| `GROWTH` | Growth Enterprises | Everything in Starter plus their own domain. |
| `PRO` | Custom Enterprises | Custom, up to a separate VPS and bespoke development. Open to any travel agency, not only PIHK or konsorsium — the tier is defined by what it offers, not by who buys it. |

**`plan` was decorative until now** — the column existed since 001 but was read
nowhere in the Go code, so no feature was gated by it. Custom domains are the
first entitlement actually enforced. If other paid features are assumed to be
gated, check: they probably are not.

Enforcement is at **resolution**, not only at creation: a verified domain stops
resolving and drops out of the CORS allowlist the moment the plan no longer
includes it. Gating only on create would leave a downgraded operator's domain
served forever. `ListMyDomains` returns `customDomainsEnabled` so the CMS can
explain the entitlement, but the server stays the authority.

**White-label foundation (Level 2)**

The owner's direction: clients will eventually bring their own domains, and
possibly their own VPS, with jamaah / tour leader / agent all signing in on the
client's domain. Levels: 1 = platform subdomain (where we were), 2 = client
domain for public pages with auth still on the apex (where we are now), 3 =
sessions issued per client domain, 4 = dedicated instance per client.

Shipped:
- **Migration 085 `operator_domains`** (`19ebaf7`). Tenancy used to be *derived*
  from the hostname; a client domain has no slug to derive, so identity is now
  stored. Additive by design — platform subdomains still resolve exactly as
  before and nothing was backfilled. Only verified rows resolve, enforced in
  SQL and in the repository.
- **Dynamic CORS** (`35b9dd8`) reading the same table, since `/register` and
  `/apply` are served on client domains and call the API from the browser.
- **Domain claim + DNS TXT verification** with a settings panel (`7b8b694`).

Three routing traps found only by testing against the standalone server, none
visible by reading the code:
1. `platformBaseHostname` returns a client domain unchanged, so the app-route
   redirect would have sent it to itself forever.
2. The legacy `/p/{slug}` redirect bounced visitors off the client domain onto
   the platform subdomain.
3. Next re-enters middleware on the rewritten path with the server's own Host,
   losing tenant identity — now carried in a request header.


**Caddy edge is built but NOT cut over** (`b901901`). `deploy/caddy/` contains
the Caddyfile, an install script, and a cutover runbook. nginx is still the live
edge; the swap changes what terminates TLS for every tenant, so it is manual and
deliberate rather than part of a deploy.

Why Caddy at all: the wildcard comes from lego over Hostinger DNS-01, which can
only cover domains in our own Hostinger account. A client's domain has DNS at
their registrar, so its certificate must come from HTTP/TLS-ALPN — per domain,
on first request. The existing wildcard is deliberately untouched: the lego
timer keeps renewing it and Caddy loads it from disk, so no DNS plugin is needed
and the proven path is not risked.

`/internal/tls-authorize` is what makes on-demand safe. Caddy asks before
issuing for an unseen hostname; a 200 authorises it, so the endpoint is exactly
as strict as routing. Without it, anyone pointing DNS at the server could make
it request certificates on their behalf and burn Let's Encrypt rate limits. Same
403 for unknown and not-entitled, so it cannot be used to probe plans.

Two things to carry into the cutover:
- **Certificate file permissions.** Caddy runs as the `caddy` user;
  `/etc/letsencrypt/live/*/privkey.pem` is usually root-only. Check with
  `sudo -u caddy test -r ...` *before* stopping nginx, or Caddy fails to start
  at the exact moment there is nothing serving.
- **X-Real-IP.** The Go rate limiter trusts it because the proxy always sets it
  itself. Caddy does not by default; the Caddyfile sets it from `{remote_host}`.
  Any future edge change must preserve that, or per-IP limits become spoofable.

Rollback is a service swap (`stop caddy && start nginx`), not a restore — nginx
keeps its config throughout. `deploy.yml` still installs the nginx config and
must only be switched to `install-caddy` after Caddy is live.

**Still open, in order**
- **Caddy + on-demand TLS.** The wildcard is issued by lego + Hostinger DNS-01,
  which cannot work for domains at a client's registrar; those need HTTP-01.
  Plan: Caddy terminates TLS, loads the existing wildcard from disk (so the
  proven renewal path is untouched) and issues on-demand certificates for
  client domains, gated by an `ask` endpoint backed by `operator_domains`.
  Deliberately a separate migration, not bundled with feature work.
- **Level 3 auth.** Better Auth is pinned to one origin and its cookie is
  host-only for the apex. `trustedOrigins` supports an async per-request
  function in 1.6.28, but was deliberately NOT widened yet — auth never happens
  on a client domain today, so trusting those origins would be risk without use.
- **Centralised OAuth handoff.** Google requires every redirect URI to be
  registered manually, so social login cannot happen directly on arbitrary
  client domains. One central callback issues a single-use, short-lived,
  origin-bound code that the client domain exchanges server-side. Never put a
  session token in a URL.
- **The app-route redirect is TEMPORARY** and inverts at Level 3. It is marked
  as such in `middleware.ts`.

**Also outstanding (user-side audit)**
- No `manifest.json` or icons: the pilgrim/leader PWAs cannot be installed.
- `favicon.ico` 404s on every origin; `<html lang="en">` on an Indonesian UI;
  `/pilgrim`, `/leader`, `/agent` all inherit the title "Operator Dashboard".

**Later the same day — storefront UI round**

- **The tenant header was never sticky.** `.tenant-scope` carried
  `overflow-x: hidden`, which forces `overflow-y` to compute as `auto` and makes
  the element a scroll container; `position: sticky` then anchored the header to
  that box instead of the viewport, so it scrolled away with the page. Measured
  at `scrollY 2500` the header sat at viewport top `-2500`. Fixed with
  `overflow-x: clip` (`2a0b83d`), which contains horizontal overflow without
  creating a scroll container. Nothing in the CSS *looked* wrong, so
  `storefront-sticky-nav.spec.ts` asserts the header's measured bounding rect
  and pins `overflow-y: visible` — reintroducing `hidden` now fails loudly.
  Any future `overflow` change on `.tenant-scope` must keep this in mind.

- **Packages section reworked** (`6018cd7`): a carousel of 4:5 poster cards
  (operators upload portrait promo flyers, so the artwork is shown whole rather
  than cropped through the price), plus a picker panel opened by a card or the
  floating shortcut — dark header, scrollable departures, expandable rows
  carrying the facilities/hotels/airline/room-price detail that previously hid
  behind a `<details>` element. Built entirely from existing proto fields; no
  CMS or schema change. Carousel arrows only render when the track overflows.

- **Contrast constraint, worth repeating:** `--tenant-brand-text` comes from
  `readableText`, a binary luminance flip calibrated against `--tenant-brand`
  exactly. A gradient that darkens below the brand colour destroys it — a first
  attempt at the contact panels did precisely that and made the copy nearly
  unreadable. Panels now only ever lighten. Screenshot the result; reading the
  CSS will not catch this.

- **Reminder about the local PWA server:** `pnpm build` overwrites
  `public/sw.js` and breaks the offline project until `e2e:pwa:build` is re-run.
  That failure mimics a real regression exactly.

Everything below shipped to production in four deploys, all green. Ordered by
what a reader most likely needs to know first.

**Live in production**

| Commit | What |
| --- | --- |
| `6224056` | Storefront media quota + asset registry (migration 084) |
| `adf392b` | `cmd/storefront-backfill` for pre-registry objects |
| `9168a23` | Ship the backfill binary in the API image |
| `8e0cfbf` | Grant the MinIO service user `s3:ListBucket` |
| `6cae083` | Tenant storefront auth links point at the apex |
| `732f755` | All app routes on a tenant host redirect to the apex |
| `dad1080` | Nav hover, floating CTA, and contact panel polish |
| `40b241b` | Playwright browser E2E suite (`apps/web/e2e/`) |
| `1086d7e`, `93af574` | Doc corrections |

**Closed**

- Browser QA now exists and passes (10 specs). See `apps/web/e2e/README.md`.
- The MinIO production rollout was already done in the `e7fef46` release; the
  earlier "still pending" note here was stale.
- The backfill ran in production and found **nothing to adopt** — the bucket is
  empty. `-apply` is not outstanding work.
- Tenant sign-in via the storefront's own links works; confirmed by the owner in
  a real browser, including a successful Google sign-in.

**Known gaps, deliberately open**

- **Real-device QA.** iOS Safari audio autoplay, and an installed PWA reopened
  offline, are still unverified. No amount of headless testing settles these.
- **The dashboard lives on the apex, by design.** `tawafiqhub.id/dashboard`, not
  `vacana.tawafiqhub.id/dashboard`. Better Auth is pinned to one origin and its
  cookie is host-only for the apex. Moving the dashboard onto tenant subdomains
  means cross-subdomain cookies, multi-origin CORS, and Better Auth changes —
  an architecture decision, not a fix. Only worth doing if full white-label is
  a business goal.
- **`readableText` is a binary luminance flip** (`TenantStorefront.tsx`). For an
  emerald brand it returns dark navy, which is only ~4.5:1 on the brand fill and
  drops below AA wherever the design applies opacity. Any future panel styling
  must not darken below `--tenant-brand`. Raising the threshold would improve
  contrast for everyone but flips existing tenants' text colour, so it needs a
  deliberate decision.
- **Native mobile apps.** `apps/mobile-leader` and `apps/mobile-pilgrim` are
  still empty scaffolds; modules 5/6 ship as PWAs.

**Two mistakes worth not repeating**

Both were the same error: verifying against a *more privileged or more
convenient* path than production uses.

1. The local API had **no `S3_*` keys**, so `storage.New` returned nil and every
   upload RPC was silently a no-op. The MinIO integration tests passed because
   they construct a `Store` directly, bypassing the API and the browser.
2. The backfill was "verified end-to-end locally" using MinIO's **root**
   credentials, which bypass the bucket policy — so the missing `s3:ListBucket`
   only surfaced as a 403 in production.

The fix in both cases was to reproduce the real conditions: real env, real
least-privilege user, real browser. Prefer that over a convenient approximation.

- **The storefront backfill is done, and there was nothing to adopt.** Run in
  production on 2026-08-26 after the deploy of `8e0cfbf`: `objects scanned: 0`,
  everything else 0. Confirmed independently with MinIO root credentials (which
  bypass the bucket policy) — `safrat-uploads` holds 0 objects, 0 bytes. No
  operator had uploaded storefront media yet, because the storefront's default
  logo/hero/gallery images are bundled Next assets served from `/images/...`,
  not objects in the bucket. `-apply` was therefore never needed and is not
  pending: every future upload is registered by the normal path. The command
  stays in the image as re-runnable maintenance, not as outstanding work.

- **The backfill's first production run failed with a 403 on ListObjectsV2.**
  The least-privilege MinIO service user had no `s3:ListBucket`. Fixed in
  `8e0cfbf` by granting it, conditioned to the `storefront/` prefix. The cause
  of the miss is worth remembering: local runs used the *root* credentials from
  `docker-compose.yml` (`safrat-local`), which bypass the policy, so the real
  permission path was never exercised. Verified the second time by creating a
  MinIO user, attaching the actual policy file, and confirming the old policy
  reproduces the exact 403 while the new one lists correctly.

- **Tenant sign-in was broken by CORS and is partly fixed** (`6cae083`). The
  tenant storefront linked to `/sign-in` relatively, so on
  `vacana.tawafiqhub.id` the page rendered on the tenant origin while Better
  Auth's client stayed pinned to `NEXT_PUBLIC_APP_URL` — every `/api/auth` call
  cross-origin and blocked. All four auth links are now absolute to the apex.
  **Still broken:** typing a tenant `/sign-in` URL directly. That needs a
  cross-host redirect from middleware, which could not be verified locally
  (Next normalizes the request host to localhost in dev and collapses even a
  genuinely cross-origin `Location` down to a bare path, looping the tenant host
  back to itself). Shipping it unverified risked looping every tenant
  storefront, so it was deliberately left out. **Browser confirmation of the
  shipped fix on a real tenant subdomain is still outstanding.**

- **Browser QA now exists.** `apps/web/e2e/` is a Playwright suite that drives
  the real local stack (Next.js, the Go API, PostgreSQL, MinIO) through a real
  browser — see `apps/web/e2e/README.md` for how to run it and what it covers.
  Eight specs pass: the full image-upload chain (presign → WebP conversion →
  PUT → verification → promotion → registry row → public read → quota), draft
  vs published isolation, a real MP3 upload with autoplay and mute persistence,
  the blocked-autoplay fallback, service worker install/precache, and a fresh
  tab served entirely from cache with the network down. It is deliberately
  outside `pnpm lint`/`typecheck`/CI because it needs running services and
  writes to the local database. Fixtures are a dedicated operator and a linked
  pilgrim, provisioned through the app's own endpoints; `e2e:clean` removes them.
  Known limits are listed in that README — autoplay blocking is injected rather
  than enforced by the browser, the cold-start test keeps one tab open, and real
  devices (iOS Safari audio, an installed PWA reopened offline) remain unverified.

- **The local API had storefront storage disabled entirely.** `apps/api/.env`
  carried no `S3_*` keys, so `storage.New` returned nil and every upload RPC was
  a no-op locally; the MinIO integration tests pass because they construct a
  Store directly and bypass the API. The keys are now set (they were already
  documented in `.env.example`), which is what made browser upload QA possible
  at all.

- **Found by that QA:** the storefront quota indicator is stale after an upload.
  `setStorageUsage` in `OperatorProfilePanel` is called from load, saveDraft and
  publish, never from the upload handler, so "N file aktif" and the used-bytes
  bar do not move until the operator saves, publishes, or reloads. Not a data
  bug — the registry and quota enforcement are correct — but the operator sees
  the wrong number for the rest of the session. `storefront-cms.spec.ts` asserts
  the current behaviour and documents it.

- The one-time inventory/backfill for pre-registry storefront objects now
  exists as `apps/api/cmd/storefront-backfill`. It pages the live `storefront/`
  prefix, resolves each key to its operator and asset kind, and adopts the
  objects into the registry as LIVE rows so their bytes count toward the
  operator quota and the cleanup worker can manage them. It only reads the
  bucket and inserts rows — it never uploads, moves, or deletes an object, and
  never modifies an existing row, so it is safe to re-run. It defaults to a dry
  run whose counts are exact, because the real statement runs in a transaction
  that is rolled back; `-apply` commits. Objects whose operator no longer
  exists, whose size is out of the registry's bounds, or whose key does not
  parse are counted and reported rather than silently skipped. **Adopting an
  object brings it under the cleanup sweep**, so an adopted object referenced by
  neither the draft nor the published snapshot is deleted after the seven-day
  recovery window — run the dry run first and read the counts. Verified
  end-to-end against local MinIO plus PostgreSQL: dry run wrote nothing,
  `-apply` adopted the object with its real size, a second `-apply` adopted
  nothing and left quota usage unchanged, and a deliberately unparsable key was
  reported and left alone. The local database and bucket were returned to their
  prior state afterwards.

- The S3 environment resolution now lives once in `storage.ConfigFromEnv()`.
  `cmd/worker` previously had its own copy that silently dropped the legacy
  `R2_*` fallbacks `config.Load()` honours; the server, the worker, and the
  backfill command now all read the same variables the same way.

- Storefront media hardening now has a PostgreSQL-backed reservation and live
  asset registry (migration 084), enforcing a configurable per-operator quota
  under an advisory transaction lock so concurrent uploads across API replicas
  cannot overrun the limit. Confirmation records the verified live object and
  stays retry-safe if PostgreSQL is temporarily unavailable. The CMS reports
  used/quota bytes, active files, and pending uploads. An hourly worker expires
  stale reservations, marks files unused only when absent from both draft and
  published snapshots, waits a seven-day recovery window, then removes the
  object before its registry row. The worker has the same least-privilege S3
  configuration as the API. Migration 084 and its quota/reference integration
  tests pass on the local PostgreSQL database; worker ordering/error tests,
  storage tests, full Go test/vet/build, frontend lint/typecheck/build, Buf lint,
  and Compose validation also pass. Existing pre-registry objects remain safe
  and unmanaged; a one-time inventory/backfill is recommended later if exact
  historical usage accounting is required.

- Storefront customization now also covers the previously hard-coded details:
  operators can upload a dedicated WebP About image with required alt text and
  caption, edit exactly four unique assurance pillars, define one to seven
  unique operating-hour groups, and choose an initial music volume from 5 to
  100 percent. Existing tenant snapshots receive compatible defaults in the
  service layer, so old published pages continue rendering without migrations.
  The public renderer uses the custom values while retaining safe bundled-image
  fallbacks. API and CMS validation agree on counts, required values, duplicate
  prevention, URL safety, and volume limits.

- Berita and Blog now have a complete draft editor: collapsible article forms,
  automatic but overridable slugs, tenant-scoped WebP cover uploads, required
  alt text, author, publication date, excerpt/body counters, per-article SEO
  title and description, and a live Google-result preview. Client and backend
  validation enforce complete articles, valid 3 to 180 character slugs, unique
  slugs across both collections, valid cover URLs, alt text, valid timestamps,
  and a maximum of 30 entries per collection. Published cards and detail pages
  show author/date metadata. Article media uses a dedicated storage kind while
  retaining the existing tenant isolation, verification, and 5 MB WebP limit.
  TypeScript, targeted ESLint, Buf lint, production web build, full Go tests,
  vet, build, and the new storage coverage pass. Signed-in CMS interaction and
  a real image upload remain recommended browser QA.

- The tenant storefront now has a transparent hero-overlay header that remains
  sticky, resolves to the tenant's semantic surface after the hero, highlights
  the active anchor with a rounded state, and exposes an accessible mobile menu.
  A floating package shortcut only appears after leaving the hero. Operators can
  configure up to eight unique social channels in the CMS, rendered as a
  theme-aware social hub. They can also upload one tenant-scoped MP3 up to 10 MB,
  enable looping background music, and set its title. Image uploads remain WebP
  capped at 5 MB. The API verifies MIME, size, tenant key, extension, and WebP or
  MP3 signature before promoting pending media. The storefront attempts autoplay,
  falls back after browser interaction when blocked, provides play/mute controls,
  and stores the visitor's mute preference locally. Production build, ESLint,
  Buf lint, storage tests, service tests, and the full Go suite pass. Interactive
  browser automation remains unavailable, so signed-in CMS and device-level
  audio autoplay QA remain recommended before production push.

- The tenant storefront now has a full CMS in Dashboard Settings with separate
  draft and published JSON snapshots, optimistic revision checks, authenticated
  preview, and an atomic publish operation. Public tenant pages only read the
  published snapshot, while preview uses the exact same renderer as production.
  Operators can edit brand/hero/contact content, package photos and descriptions,
  facilities, itinerary, gallery with required alt text, testimonials, and FAQ.
  Package choices remain tied to active operational seasons, so CMS content cannot
  invent or duplicate a season.
- Storefront media uploads now go directly from the browser to an S3-compatible
  bucket through a tenant-scoped, 10-minute presigned PUT. The browser redraws
  images to strip metadata, resizes them, and creates WebP before upload; the API
  then HEADs, downloads, signature-checks, fully decodes, and dimension-checks the
  object before returning its usable public URL. Local development uses MinIO on
  `:9000` (console `:9001`) with tested browser CORS. Production is now designed
  around self-hosted MinIO on the existing VPS rather than Cloudflare R2. The
  versioned Compose/bootstrap/Nginx setup uses a persistent volume, a separate
  least-privilege API user, global apex-only CORS, public reads only under
  `storefront/`, and no exposed admin console. Upload tickets target the
  `storefront-pending/` prefix and confirmation promotes verified images to
  `storefront/`; a 1-day lifecycle can therefore remove abandoned uploads
  without ever expiring published media. The real production-isolated MinIO
  integration passed CORS, signed upload, full WebP verification, promotion,
  anonymous published read, pending privacy, cleanup, and repeatable bootstrap.
  VPS secrets and the first production rollout still need to be completed.
- Migration 082 creates `operator_storefronts` and seeds every existing operator's
  legacy public profile into draft and published revision 1. The migration is
  applied locally. Repository integration tests prove draft isolation, atomic
  publish, and stale-tab conflicts; storage integration tests prove real presign,
  CORS preflight, WebP upload, verification, and cleanup against MinIO. The QA
  fixture cleanup ordering was corrected and the local database is empty again.
- The rich tenant renderer keeps TawafiqHub attribution and adds package details,
  itinerary, gallery, testimonial, and FAQ sections while retaining brand-driven
  light/dark styling. Local tenant-host smoke testing returned HTTP 200 and found
  all representative CMS sections in server-rendered HTML. In-app browser
  automation was unavailable in this session, so a signed-in visual interaction
  pass for CMS editing/upload/preview remains recommended.
- The tenant subdomain root is now a full white-label travel storefront rather
  than a compact public-profile card. Migration 081 adds one brand color plus
  editable hero eyebrow/title/subtitle/image fields; the existing operator
  name, logo, description, contact, legal details, and future seasons populate
  the remaining template. Dashboard settings exposes all editable values, the
  storefront derives accessible foreground contrast from the chosen color,
  supports light/dark mode, keeps package registration slugs unchanged, and
  permanently attributes "Powered by TawafiqHub". A 97 KB default Umrah hero
  WebP is bundled for operators without photography. Migration 081 is applied
  locally and the local database was returned to its prior empty state after a
  non-persisting storefront QA fixture. Production rollout remains pending an
  explicit push/deploy request.

- The platform apex `https://tawafiqhub.id` is the canonical app origin.
  Root/www TLS and DNS reach the VPS; the version-controlled nginx config
  proxies the apex and permanently redirects the old `app` host. The promotion
  helper owns the two historical active VPS targets (`tawafiqhub` and the now
  neutral `tawafiqhub-root`) with validation and two-file rollback. The deploy
  workflow also smoke-tests public apex, service worker, API, exact redirects,
  and CORS so inactive-config regressions cannot produce a green deployment.
  Better Auth, CORS, build URLs, and deployment defaults use the apex. The VPS
  deploy script re-exports the canonical CORS origin after sourcing `.env.prod`
  so a stale persisted `app` value cannot override the compose default. The VPS
  already has the corrected root-owned `safrat-install-nginx` helper and its
  single-command sudoers rule.
  Before production rollout, add
  `https://tawafiqhub.id/api/auth/callback/google` to the Google OAuth client's
  authorized redirect URIs. Existing host-only sessions on `app` will require
  a one-time sign-in on the apex.
- The hardening continuation fixes all proto RPC request naming violations;
  `buf lint` is now clean while RPC paths and field numbers stay wire-compatible.
- Group-city (both admin and Muttawwif entry points), kloter-status, and ritual
  bulk producers now commit their authoritative writes and outbox events in one
  PostgreSQL transaction. Migration 079 adds 30-second worker leases and bounded
  exponential retry backoff to prevent concurrent duplicate dispatch.
- Firebase push methods return errors and retry transport failures immediately
  (100ms/250ms backoff within a 4-second budget). SOS stays on the direct path;
  alert persistence never rolls back if push ultimately fails, and Sentry records
  the failure. Outbox delivery now receives push errors and can retry correctly.
- Redis now backs the shared operator cache and distributed token-bucket rate
  limiter in addition to monitoring pub/sub. Operator updates publish cross-replica
  L1 invalidations; Redis failure falls back to PostgreSQL/local limiting.
- All 23 historical React Hook warnings were resolved with stable callbacks and
  memo dependencies. ESLint now reports 0 errors and 0 warnings.

- The cold-start offline item below is committed as `c76f460` with Serwist 9:
  `app/sw.ts` is the source and production builds generate the ignored
  `public/sw.js`. All 20 `/pilgrim` + `/leader` routes and build assets are in
  the precache manifest.
- Firebase Messaging is bundled into that same production root-scope worker;
  `/firebase-messaging-sw.js` remains a development-only fallback. This fixes
  the prior collision where the cache worker and Firebase worker replaced each
  other at scope `/`.
- `RequireAccess` now has a bounded 72-hour offline access snapshot, and leader
  groups are read-through cached. Without these, a precached shell still
  redirected to sign-in during a cold offline start.
- Operator slugs now use the meaningful full name after removing generic legal
  prefixes (`PT`, `CV`, `KBIH/KBIHU`, etc.). New onboarding lets the owner edit
  that suggestion, previews `{slug}.tawafiqhub.id`, and checks availability in
  real time. Migration 076 repairs only existing generic slugs such as `pt`; it
  is applied locally.
- The operator-subdomain root is now the canonical public profile URL. The old
  `/p/{slug}` address permanently redirects to `{slug}.tawafiqhub.id/`; share
  buttons use the tenant URL, and package CTAs use season slugs instead of UUIDs.
  API/database uniqueness remains the final race-safe guard for chosen slugs.
  The wildcard continuation is implemented locally: shared frontend hostname
  parsing, reserved platform slugs, migration 080's database constraint,
  wildcard Nginx routing, automated Hostinger DNS-01 certificate renewal, and
  deploy smoke tests. It is intentionally not on production yet. Hostinger DNS
  `A * -> 103.179.66.25` is now active and verified through Cloudflare, Google,
  and both authoritative nameservers. Before any push to `main`, bootstrap the
  root-only `lego` certificate/timer from a staging worktree and reinstall the
  updated Nginx helper in the exact order documented in `DEPLOY.md`.
  ACME prechecks timed out despite both TXT values being confirmed on both
  Hostinger authoritative nameservers plus Cloudflare and Google. The renewal
  script now uses lego v5's fixed 120-second DNS wait, bypassing that false
  negative before Let’s Encrypt performs the authoritative validation.
  The apex plus wildcard certificate was issued successfully on the VPS on
  2026-08-24, its key/SAN/expiry checks passed, and the daily
  `safrat-wildcard-tls.timer` is active. Production wildcard routing still
  requires the reviewed release to reach `main` so the deployment workflow can
  promote the version-controlled Nginx configuration.
  Local verification passed: migration 080 plus a non-persisting reserved-slug
  constraint probe, all Go tests/vet/build, web lint/typecheck/production build,
  `buf lint`, shell syntax, Dockerized `nginx -t`, and Host-header routing
  (`tenant-probe.localhost` 404; legacy `/p/tenant-probe` 308).
- Landing hero messaging now sells one end-to-end operational control surface
  from Indonesia to Saudi, with gold-gradient emphasis and off-white dark-mode
  headings. FAQ dark mode separates active questions in warm gold from muted
  slate answers using unlayered semantic CSS, avoiding the Tailwind cascade
  issue that left accordion cards white. Footer office coverage is DKI Jakarta
  and Kota Bekasi.
- Season creation is idempotent and protected at three layers: synchronous UI
  submit locks, backend exact-retry upsert, and a unique normalized season name
  per operator. Migrations 077–078 safely remove empty duplicates. Same-name
  rows with dependent data make the migration fail for manual merge rather
  than cascading data loss.
- Verified locally: web typecheck, ESLint (0 errors; 0 warnings), production
  build, and generated-manifest inspection (20/20 PWA
  routes present). A real-browser/device offline test is still recommended.
- The apex-domain deployment through `dfabc98` was pushed and deployed
  successfully on 2026-08-24 (GitHub Actions run `32711294125`).

## Repo / deploy state

- The production release through `e7fef46` is on `origin/main` and passed CI,
  image builds, migrations, VPS restart, and public smoke tests in Actions run
  `32916437921`.
  **Pushing `main` triggers a production deploy** (`.github/workflows/deploy.yml`
  → builds images, runs goose migrations, installs validated nginx config, and
  redeploys `tawafiqhub.id`). Do not push again without explicit owner approval.
- Generated code (`apps/api/internal/gen`, `packages/proto-gen`) is **gitignored**
  and rebuilt by CI — never commit it. `apps/web/tsconfig.tsbuildinfo` and
  untracked scratch `*.md` / media are also excluded.
- **Local dev DB was wiped clean** (all rows truncated, schema kept) for fresh
  manual testing. Migrations **073–084 are applied locally**; in prod goose
  applies them on deploy.
- Local processes: web dev on `:3131`; Go API on `:8131`. Both are expected to
  be restarted from current source after the latest local commit.

## What shipped this session (the 5 commits)

1. `f97ad06` Landing: Masuk/Daftar prioritized, demo flow removed, WA contact
   `+62 812-8303-1003`.
2. `65ea474` Fix transparent mobile menu drawer (was trapped by the header's
   `backdrop-blur` containing block).
3. `d3a1d69` **Onboarding wizard + operator public profile** — migration 073;
   `OperatorService.UpdateMyProfile` (auth) + `GetPublicProfile` (public);
   tenant-root public profile (internally rendered by `/p/[slug]`); settings
   editor + share link; post-onboarding banner.
4. `f9dc1c8` **Production hardening**:
   - **#1 Transactional outbox** (migration 074 `cascade_events`): producers
     enqueue in the same tx as the write; worker relay (`cascade:dispatch`
     `@every 10s`, `FOR UPDATE SKIP LOCKED`, dead-letter after 5 attempts)
     drains it. **Health-report BERAT push** migrated as the atomic reference.
   - **#2 Redis-backed event bus** (`internal/events/bus.go`): same interface,
     picks Redis when `REDIS_URL` set → cross-replica. In-memory path unchanged.
     `docker-compose.prod.yml` api service now sets `REDIS_URL`.
5. `472e54d` **Offline hardening**:
   - Poison-safe write queue (`lib/offline.ts`): per-item attempts + dead-letter
     so one failing item can't wedge the SOS queue; idempotency-key plumbing.
   - **SOS create idempotency** (migration 075): `idempotency_key` + partial
     unique index; `ON CONFLICT DO NOTHING` + `created` flag → replay returns the
     existing alert without re-notifying. Verified end-to-end.

## Roadmap (prioritized) — with the analysis already done

### Completed locally, pending browser verification
- **#3 Precache for cold-start offline.** Implemented in the committed
  continuation described above. The remaining validation gap is a real-browser
  test: load each PWA online once, close it, enable offline mode, then cold-open
  the installed PWA and exercise cached reads/queued writes.

### Completed in the hardening continuation
- Group-city, kloter-status, and ritual bulk cascades use the transactional
  outbox; SOS remains direct with bounded fast retry.
- Operator cache invalidation and public RPC rate limits are Redis-distributed,
  with bounded local/DB fallbacks for availability.

### Skip / already handled
- **Check-in idempotency** — redundant: `check_ins` already has
  `UNIQUE(movement_id, pilgrim_id, type)`.
- **Chat idempotency** — possible but low value (duplicate message only).

## CI note
- `buf lint` is clean. Request message names are now method-specific; generated
  clients must be rebuilt with `pnpm buf:generate` (CI already does this).

## Local verify recipe
- Go: `cd apps/api && go build ./... && go vet ./... && go test ./...`
- Web: `pnpm --filter @hajj-saas/web typecheck && (cd apps/web && npx eslint .)`
- Redis cross-instance tests: `REDIS_TEST_URL=redis://localhost:6380 go test ./internal/events/ ./internal/middleware/ ./internal/repository/`
- Backend smoke tests run a throwaway server on `:8132` against the local DB
  (`PORT=8132 go run ./cmd/server`) — insert a temp operator, curl the RPC, clean up.

---

## Lanjutan: harga berlapis + pembeli agen

Berhenti di tengah karena limit. Yang ada di `main` **build bersih, vet bersih,
tes hijau** — tidak ada yang setengah jadi di dalam pohon. Yang kurang adalah
lapisan berikutnya yang belum ditulis sama sekali, bukan yang rusak.

### Model harganya

Harga dibangun ke atas, bukan dibagi ke bawah:

```
base_price_idr        harga TawafiqHub ke travel   (products.base_price_idr)
+ operator_markup     ditambah travel              -> HARGA AGEN
+ agent_markup        level agen                   -> HARGA JAMAAH
```

Konsekuensi yang harus dipertahankan siapa pun yang melanjutkan:

- **Agen beli untuk diri sendiri tidak dapat komisi** — bukan aturan tambahan,
  tapi karena harga agen memang tidak mengandung level agen. Ada CHECK di
  `orders_agent_buyer_earns_no_commission_check` dan
  `orders_agent_buyer_pays_no_agent_markup_check`.
- **Jamaah tanpa perujuk membayar sama** dengan yang punya perujuk. Level agen
  tetap ditagih; kalau tidak ada perujuk, jatuh ke travel. Kalau ini diubah,
  agen jadi menghukum pelanggannya sendiri.
- **Markup Rp0 itu keputusan, markup tidak ada itu kekosongan.** `Configured`
  membedakan keduanya; produk yang belum diatur ditolak saat dijual
  (`ErrMarkupUnset`), tidak dijual seharga markup saja.
- **Tidak ada pembulatan.** Harga adalah penjumlahan. Uji
  `TestNothingIsEverLostToRounding` menjaga komponen dan penyelesaian sama-sama
  kembali ke total. Skema basis-point yang lama kehilangan 1 rupiah pada
  sekitar 1 dari 200 order.

### Yang sudah jadi dan teruji

- Migrasi `110_order_buyers.sql`, `111_layered_pricing.sql` — naik dan turun
  keduanya sudah diverifikasi terhadap Postgres lokal.
- `domain.PriceLevels`, `service.ComputePrice` + 9 tes.
- Query sqlc: `GetProductPricing`, `UpsertProductMarkup`, `ListProductMarkups`,
  `SetProductBasePrice`.
- `ProductRepository.Pricing` / `SetMarkup` / `SetBasePrice` / `ListPricing`.
- Ketiga jalur pembelian jamaah (`CreateOrder`, `CreateManualOrder`, dan
  `CreateOrderForPilgrim`) sudah memakai `ComputePrice` dari satu pembacaan
  atomik produk + markup. `computeSplit` dan `order_split_test.go` sudah
  dihapus.
- Order jamaah baru membekukan `buyer_kind`, harga unit, base, markup travel,
  markup agen, total, dan tiga nilai settlement. Tes integrasi PostgreSQL
  memeriksa nilai-nilai itu pada row sebenarnya, bukan hanya hasil fungsi.
- Batas modal supplier sekarang dibandingkan dengan base milik TawafiqHub,
  bukan total pelanggan: markup travel/agen tidak boleh menyamarkan kerugian
  platform.

### Yang belum — urutan yang saya sarankan

1. **Jalur beli untuk agen sudah selesai** — proto, handler, buyer-safe reads,
   destination yang dibekukan, integration test, dan tab UI `/agent` sudah ada.
2. **Layar markup untuk travel** dan **layar harga dasar untuk admin** belum
   ada. Perhatikan: harga dasar sengaja *tidak* lewat `UpdateProduct`, supaya
   travel tidak bisa menggeser harga yang mereka bayar.

### Keputusan owner yang sudah diambil tapi belum dikerjakan

4. **PPOB tetap aktif**, produk tanpa routing harus menjawab jelas — "Produk
   belum diatur routing-nya" — dan ditolak **sebelum** pembayaran dibuat, bukan
   sesudah. Belum dikerjakan.
5. **Batas Rp20 juta per akun per hari, produk digital saja.** Harus ditegakkan
   di database (unique/exclusion atau agregat berkondisi), bukan cek-lalu-tulis
   — dua permintaan bersamaan lolos dari cek-lalu-tulis. Belum dikerjakan.
3. **Katalog produk digital pindah ke kepemilikan platform.** Belum dimulai;
   `product_markups` sengaja tabel terpisah supaya perpindahan itu tidak perlu
   membongkar apa pun.
6. KYC manual — tidak ada pekerjaan. 7. HA — menyusul.

### Yang masih menunggu tindakan owner

- `KYC_ENCRYPTION_KEY` belum ada di `.env.prod`. Tanpa itu penulisan KYC
  ditolak. Bikin dengan `openssl rand -base64 32` **di terminal sendiri**,
  jangan di sesi bersama agen — outputnya masuk transkrip dan riwayat shell.
  Simpan di Bitwarden bersama sidik jarinya (tercetak di log start server).
- `XENDIT_WEBHOOK_ALLOWED_IPS` — harus diminta ke support Xendit.
- Cutover role DB non-superuser (`DEPLOY.md §12b`) dan cutover Caddy
  (`deploy/caddy/README.md`) — keduanya manual, belum dijalankan.
- Ada commit lokal yang belum di-push. Jangan simpan angka tetap di handoff:
  verifikasi saat melanjutkan dengan
  `git rev-list --count origin/main..HEAD` karena commit dokumentasi yang
  memperbarui angka itu sendiri langsung membuat angkanya basi.

### Satu hal yang sengaja saya tinggalkan

`orders.operator_id` dan `orders.season_id` masih `ON DELETE CASCADE`.
`pilgrim_id` sudah saya ubah jadi `RESTRICT` — cascade-nya akan menghancurkan
seluruh riwayat transaksi satu jamaah. Menghapus satu tenant utuh adalah
keputusan berbeda dan butuh tujuan arsip dulu sebelum delete-nya bisa ditolak.

---

## Layar harga & markup (selesai)

Sampai commit ini, harga berlapis hanya bisa diubah lewat SQL langsung. Sekarang
dua level itu punya pemiliknya masing-masing, dan sengaja dipisah:

- **Travel** — `/dashboard/products/harga`. Mengatur `operator_markup_idr` dan
  `agent_markup_idr`. Melihat harga dasar tapi tidak bisa mengubahnya.
- **Platform** — tab Produk di `/admin`. Mengatur `base_price_idr`, di sebelah
  harga modal supplier. Scoped by product id saja, bukan per operator: ini harga
  yang travel *bayar*, dan travel yang bisa mengubahnya bisa menjual di bawah
  yang ditagihkan ke mereka.

### Yang harus dipertahankan

**Harga dihitung setiap kali dibaca, tidak pernah disimpan.** Harga tersimpan
adalah salinan dari turunan; begitu level di bawahnya bergeser, dua angka
berselisih dan tidak ada yang bisa bilang mana yang jadi hak pelanggan.

**Kelayakan jual diputuskan dengan menjalankan gate checkout yang asli**
(`pricePilgrimOrder` / `priceAgentOrder`), bukan dengan menyalin syaratnya ke
layar. Layar yang punya pendapat sendiri akan menyimpang, dan penyimpangannya
baru terlihat saat pelanggan sudah mencoba bayar.
`TestPricingScreenAgreesWithCheckout` menjaganya di lima konfigurasi.

**Respons `SetProductMarkup` membaca ulang dari database**, bukan menggemakan
request. Menghitung dari yang baru dikirim akan menampilkan input pemakai
sebagai hasil — termasuk saat harga dasarnya kosong dan produknya tetap tidak
bisa dijual.

### Dua hal yang ikut berubah

Antrean celah platform sekarang memunculkan produk yang **kekurangan harga
dasar**, bukan hanya yang kekurangan harga modal. Sebelumnya ia melaporkan
"semua beres" sementara produk terdiam tak bisa dijual.

Kolom "Harga Jual" lama dihapus dari tab admin. Dengan pembagian yang sudah
dibalik, itu angka tersimpan yang tidak lagi jadi dasar harga siapa pun —
membiarkannya berarti menampilkan harga yang tampak otoritatif padahal tidak
ada pembeli yang membayarnya.

### Artefak build

`apps/api/server` (26 MB) dan `apps/web/tsconfig.tsbuildinfo` tidak lagi
dilacak. **Keduanya tetap ada di riwayat** — binernya ada di setiap commit sejak
commit pertama dan sudah ada di `origin`, jadi objeknya menetap. Mengeluarkannya
berarti menulis ulang riwayat yang sudah dipublikasikan.

### Belum dikerjakan

Antrean berikutnya tidak berubah: PPOB menolak checkout tanpa routing aktif,
batas Rp20 juta per akun per hari, lalu katalog produk digital pindah ke
kepemilikan platform.

---

## Gate routing & batas harian (selesai)

Dua keputusan owner, nomor 4 dan 5.

### Routing: menolak sebelum uang diambil

Produk PPOB/roaming tanpa jalur ke supplier dulu bisa dibeli — uang diambil,
fulfilment dibuka, dan baru saat itu ketahuan tidak ada tujuan pengiriman.
Sekarang ditolak **sebelum** pembayaran dibuat.

Status routing dibaca bersama harga dalam satu query, bukan query terpisah,
supaya layar harga dan checkout menilai baris yang sama. Ia mengalir sebagai
data ke gate harga yang sudah ada, jadi layar travel menampilkan penolakannya
otomatis dan katalog agen menjatuhkan produknya.

**Tiga penolakan, bukan satu**: belum ada routing, routing dinonaktifkan,
supplier tidak aktif. Masing-masing kesalahan berbeda. Dan **semuanya menyebut
TawafiqHub** — routing milik platform, travel tidak bisa memperbaikinya
sendiri, jadi penolakan tanpa arahan adalah jalan buntu.

Paket perjalanan dan perlengkapan dikecualikan: keduanya membuka fulfilment
tapi diselesaikan manusia, tidak ada panggilan supplier untuk dirutekan.

### Batas Rp20 juta: ditegakkan database

Bagian sulitnya bukan angkanya. Batas yang dicek dengan membaca total lalu
menulis order bukan batas: dua permintaan bersamaan sama-sama membaca total
lama, sama-sama menemukan ruang, sama-sama menulis.

Jadi totalnya hidup di baris dengan CHECK, dan pembelanjaan adalah UPSERT yang
menambah di tempat. **Dibuktikan, bukan diasumsikan**: enam pembelian Rp9 juta
serentak terhadap batas Rp20 juta menyisakan tepat dua yang lolos.

Order dan konsumsi batas satu transaksi. Konsumsi dulu → kuota bocor saat
insert ternyata pengulangan. Insert dulu → order tercipta yang seharusnya
ditolak.

**Harinya Asia/Jakarta**, bukan UTC. Hari UTC berganti pukul 07:00 waktu
setempat.

Refund/gagal/kedaluwarsa mengembalikan kuota, dalam satu statement yang
sekaligus menghapus capnya — jadi idempoten di tiga jalur settlement dan sweep
tanpa perlu koordinasi.

### Retensi: 30 hari (sudah diputuskan dan dikerjakan)

Owner memutuskan **satu bulan** (revisi dari 3 hari, yang terlalu pendek —
sengketa tagihan minggu lalu tidak punya apa pun untuk direkonsiliasi). Siklus
backup dan shrink database menyusul.

Batasnya, disebut eksplisit karena "sebulan" ambigu: baris dihapus begitu
tanggalnya lebih dari 30 hari di belakang hari ini — baris tepat di hari ke-30
masih disimpan, hari ke-31 hilang.

Angkanya hidup di `domain.DailySpendRetentionDays`, **dan** sebagai default di
dalam `purge_daily_digital_spend` (migrasi 117). Keduanya harus bergerak
bersama: panggilan manual saat insiden memakai default database, bukan konstanta
Go.

**Penghapusnya bukan privilege, tapi fungsi `SECURITY DEFINER`.** Migrasi 115
mencabut DELETE pada tabel ini dari role aplikasi karena menghapus satu baris
mengembalikan kuota harian penuh — dan kebutuhan akan purge tidak membuat itu
jadi kurang benar. Jadi `purge_daily_digital_spend(keep_days)` menyimpan cutoff
di dalam dirinya: aplikasi bisa meminta baris kedaluwarsa dihapus dan tidak
bisa menjangkau apa pun selain itu.

- `keep_days` di-clamp minimal 1, jadi pemanggil yang meminta "jangan simpan
  apa pun" tetap tidak bisa menghapus hari ini.
- `search_path` dipatok — fungsi definer tanpa itu adalah cara menjalankan kode
  sebagai pemiliknya.
- Diuji sebagai role `safrat_app` terhadap database sungguhan: DELETE langsung
  ditolak, panggilan fungsi berhasil, baris hari ini selamat dari purge dengan
  `keep_days=0`.

Task-nya berjalan **tiap jam**, bukan harian: job harian yang terlewat karena
worker sedang restart menunggu sehari penuh, dan menjalankan ini terhadap nol
baris tidak berbiaya.

### Database uji

Tes integrasi butuh `STOREFRONT_TEST_DATABASE_URL` dengan tabel Better Auth.
Cara tercepat membuatnya:

```
docker exec safrat-postgres-1 createdb -U safrat -T safrat safrat_limit_test
```

Klon dari database dev, karena `goose` sendiri tidak bisa membuat tabel
`"user"` — Better Auth yang memigrasikannya.

---

## Katalog digital pindah ke platform (selesai)

Keputusan owner nomor 3.

`products.operator_id IS NULL` berarti milik TawafiqHub. NULL, bukan baris
operator boneka — operator palsu muncul di setiap hitungan, daftar, dan join
yang menelusuri operators, dan satu filter yang terlupa akan menampilkannya ke
pelanggan.

### Aturan yang harus dipegang siapa pun yang melanjutkan

**BACA MELEBAR, TULIS TIDAK PERNAH.**

```
baca untuk menjual   operator_id = $1 OR operator_id IS NULL
tulis milik travel   operator_id = $1
```

Lupa melebarkan bacaan → produk platform tersembunyi. Terlihat, mengganggu,
tidak berbahaya. Melebarkan tulisan → satu travel bisa mengedit katalog yang
dijual semua travel lain. Hanya satu dari keduanya yang bisa dipulihkan.

`ProductRepository` punya dua pintu yang sengaja dipisah: `GetByID` melebar
(untuk membaca), `GetOwnedByID` ketat (untuk sebelum menulis). Satu query yang
menjawab dua pertanyaan adalah cara yang salah dipakai.

### Tiga cacat nyata yang ini munculkan

Ditemukan tes, bukan penalaran:

1. **Settlement** membaca produk order yang sudah dibayar lewat `GetByID`.
   Kalau tetap ketat, jamaah membayar pulsa lalu fulfilment tidak menemukan
   apa yang mereka beli.
2. **Empat query** menyambung products ke operators dengan inner join, yang
   menjatuhkan setiap produk platform diam-diam. Itu sebabnya order berbayar
   tidak menemukan routing-nya sendiri.
3. **Harga modal supplier**, manual maupun observed, di-scope `operator_id` —
   jadi tidak pernah bisa tersimpan untuk produk platform, satu-satunya produk
   yang punya supplier.

### Musim

Produk platform tidak punya musim, agen juga tidak. Tapi order punya: laporan
operator di-scope per musim, dan order tanpa musim akan berhenti muncul di
sana. Jadi jatuh ke musim aktif operator, dan ditolak dengan penjelasan kalau
belum ada.

### Cleanup fixture: urutannya menanggung beban

`product_routes` menahan `suppliers`, `orders` menahan `products`, keduanya
RESTRICT. Satu statement gagal me-rollback seluruh cleanup — meninggalkan
semuanya, bukan sebagiannya. Urutan yang benar:

```
operators (cascade orders, pilgrims)
  -> products (cascade routes, markups)
    -> suppliers
```

Kebocoran supplier di `platform_http_test` sudah ada sejak tes itu ditulis dan
gagal diam-diam setiap run karena errornya dibuang.

### Belum dikerjakan

- **UI admin untuk `SavePlatformProduct`** belum ada. RPC-nya jalan; belum ada
  layar yang memanggilnya, jadi produk platform masih dibuat lewat SQL.
- Produk digital per-travel yang lama **di-grandfather** (`NOT VALID`), bukan
  dihapus — order menunjuk ke sana. Migrasikan manual setelah ordernya lewat.

---

## Layar katalog platform + verifikasi browser (selesai)

Katalog platform sekarang punya layarnya: tab **Katalog** di `/admin`. Sebelum
ini RPC-nya jalan tapi tidak ada yang bisa menjangkaunya — produk platform hanya
bisa dibuat lewat SQL.

Daftarnya pakai RPC sendiri (`ListPlatformCatalogue`), bukan memfilter antrean
harga modal lintas-tenant di klien. Yang kedua bekerja sampai katalognya
melewati batas 500 baris, lalu diam-diam mulai menyembunyikan baris.

### Yang hanya ditemukan browser

1. **API harus dibangun ulang** — handler-nya ada di source, tidak ada di biner
   yang berjalan, dan layarnya menampilkan `[unimplemented] HTTP 404`. Suite Go
   yang hijau tidak mengatakan apa pun tentang apa yang sedang mendengarkan.
2. **Asersi rupiah saya salah** — `Intl` menaruh spasi non-breaking setelah
   simbol, jadi layar merender `Rp 10.500` dan tes mencari `Rp10.500`. UI-nya
   benar, tesnya yang salah.
3. **Fixture E2E `portal-screens`** masih membuat produk digital milik travel —
   kelas yang sama dengan fixture Go yang sudah diperbaiki, tapi terlewat.

### Tiga spec yang ternyata sudah rusak

`storefront-cms` dan kedua spec audio gagal **sejak 2FA diwajibkan**
(commit `bfbd0a9`). Ketiganya menggerakkan layar staf tapi tidak pernah diberi
langkah pendaftaran 2FA, jadi setiap tes di dalamnya mendarat di halaman
enrolment. Kerusakan diam itu menyembunyikan regresi nyata di tes upload CMS.

Pasangan enrol/restore sekarang dibagi dari `fixture.ts`, karena ia **hanya
bekerja sebagai pasangan**: akun yang sudah enrol tidak bisa login lagi (Better
Auth menjawab tantangan TOTP, bukan sesi), jadi lupa memulihkannya membuat run
berikutnya menyimpan storage state kosong.

### Belum selesai: offline-pwa

**Masih gagal, dan saya tidak memperbaikinya.** Yang sudah saya pastikan:

- manifest precache berisi rute yang dicari tes (`/leader`, `/leader/check-in`, …)
- setiap aset yang didaftarkannya menjawab 200
- service worker-nya aktif

Tapi cache-nya terbaca kosong. Itu kegagalan instalasi saat runtime yang butuh
konsol browser untuk didiagnosis. Tidak satu pun pekerjaan di sini menyentuh
service worker, jadi ini dilaporkan, bukan diklaim.

### Menjalankan E2E

Butuh **dua** server: dev di `:3131` (dipakai setup dan hampir semua proyek) dan
build produksi di `:3141` untuk `offline-pwa` — Serwist dinonaktifkan di bawah
Turbopack. Build-nya `output: standalone`, jadi `next start` **tidak** cocok:

```
node .next/standalone/apps/web/server.js   # PORT=3141
```

`public/` dan `.next/static` harus disalin ke dalam direktori standalone dulu.

**Jangan `rm -rf .next` selagi dev server berjalan** — saya melakukannya dan
mematikan server yang sedang dipakai setup, yang muncul sebagai
"fixture sign-in failed (500)" dan terlihat seperti bug autentikasi.

---

## Dua penutup: lockout Google, dan offline-pwa

### Akun Google terkunci dari dashboard (diperbaiki)

`agilalidrus89@gmail.com` hanya punya provider `google` — Better Auth tidak
menulis baris `credential` untuk akun yang dibuat lewat Google.

Rantainya: staf wajib 2FA sebelum dashboard terbuka → `/keamanan` meminta kata
sandi akun → akun itu tidak pernah punya kata sandi → **tidak ada jalan maju**.
Pemilik tidak bisa masuk ke panelnya sendiri.

Yang membuatnya lebih sulit dikenali sebagai bug: teks di halaman itu
menjanjikan kebalikannya — *"Login lewat Google tidak terpengaruh"*. Itu niat,
bukan perilaku, dan menuliskannya membuat jalan buntu terbaca seperti salah
paham pembaca.

Sekarang halaman itu memeriksa jenis akun sebelum memutuskan apa yang diminta.
Tanpa kata sandi, ia menawarkan mengirim tautan untuk membuatnya — alur reset
Better Auth **membuat baris kredensial yang hilang**, jadi ia sekaligus jadi
pembuatan pertama kali. Login Google tetap bekerja; ini menambah cara masuk,
bukan menggantikan.

Diuji di browser dengan menghapus baris kredensial fixture dan memulihkannya di
`finally` — tanpa baris itu fixture tidak bisa login dan run berikutnya menyimpan
storage state kosong.

### offline-pwa (diperbaiki — bukan bug service worker)

`e2e:pwa:api` keluar seketika dengan `BETTER_AUTH_SECRET is required`. Server Go
membaca environment variable biasa dan tidak memuat `.env` sendiri; skripnya
hanya menyetel `PORT` dan `CORS_ALLOWED_ORIGIN`. Sekarang ia mem-`source`
`apps/api/.env` dulu.

Yang menyamarkannya: **:8141 tetap menjawab** — proses server dari sesi lama
masih terikat di port itu dengan origin berbeda. Jadi healthz 200, preflight
403, dan spec-nya gagal dengan registrasi service worker kosong. Itu terlihat
seperti masalah service worker dan sama sekali bukan.

Cara membedakannya:

```
curl -X OPTIONS http://127.0.0.1:8141/hajj.v1.IdentityService/GetMyAccess \
  -H 'Origin: http://127.0.0.1:3141'     # harus 204, bukan 403
```

**Seluruh suite browser sekarang hijau: 22 tes.**

### Peringatan untuk yang melanjutkan

Repo sudah punya skrip PWA yang benar (`e2e:pwa:build` dengan `distDir` sendiri,
`e2e:pwa:serve`, `e2e:pwa:api`). **Pakai itu.** Saya berimprovisasi — membangun
ke `.next` dan menyajikannya dengan `next start` — dan menabrak setiap masalah
yang sudah dijelaskan README: dev server dan build produksi berbagi `.next`,
`public/sw.js` ditimpa setiap build, dan API hanya menerima satu origin CORS.

---

## Enrolment 2FA Google: OTP email + QR (2026-08-27)

Alur tautan pembuatan password pada bagian sebelumnya sudah diganti. Akun yang
punya credential tetap mengonfirmasi password; akun Google-only menerima OTP 6
digit ke email terverifikasi lalu langsung memasang authenticator tanpa membuat
password lokal.

Pengaman penting berada di server, bukan hanya UI:

- OTP berlaku 5 menit, disimpan sebagai HMAC, maksimal 5 percobaan, dan baru
  dapat dikirim ulang setelah 60 detik.
- OTP yang benar membuat grant 5 menit dan terikat pada token sesi. Grant dapat
  dipakai ulang hanya dalam jendela pendek tersebut agar respons enable yang
  hilang dapat dicoba lagi tanpa email baru. Hook Better Auth tetap menolak
  `/two-factor/enable` untuk akun passwordless jika grant ini tidak ada, jadi
  endpoint bawaan tidak dapat dipanggil langsung untuk melewati OTP.
- `twoFactor({ allowPasswordless: true })` hanya membuka jalur setelah hook di
  atas mengizinkannya. QR berasal dari `totpURI`; kode pertama dari aplikasi
  authenticator tetap wajib diverifikasi sebelum 2FA aktif.
- Draft QR dan backup code disimpan sementara di `sessionStorage` tab selama
  maksimal 15 menit. Ini menjaga langkah QR tetap tampil jika session/auth
  me-remount halaman; draft dihapus segera setelah kode authenticator berhasil.

Email transaksional sekarang memakai SMTP Hostinger melalui Nodemailer. Sebelum
deploy, isi `SMTP_USER`, `SMTP_PASSWORD`, dan opsional `SMTP_FROM_EMAIL` di
`.env.prod`; default host/port adalah `smtp.hostinger.com:465` dengan TLS.

---

## Siklus 2FA lengkap: Google, backup code, dan pengelolaan (2026-08-28)

Klaim "setiap login" sebelumnya belum benar: plugin Better Auth hanya menahan
login credential; callback Google langsung menerbitkan sesi penuh. Sekarang
`twoFactorSecurity` menangani callback Google untuk akun yang sudah enrol:

- sesi OAuth yang baru dibuat dihapus dari database **dan** seluruh Set-Cookie
  sesi dibersihkan sebelum respons meninggalkan server;
- server membuat signed pending challenge yang sama dengan jalur credential,
  lalu mengarahkan ke `/two-factor-challenge`;
- TOTP atau satu backup code yang valid baru menerbitkan sesi aplikasi.

Login credential dan Google memakai komponen challenge yang sama. UI sekarang
menawarkan backup code (format `XXXXX-XXXXX`); kode bersifat sekali pakai.
Respons 401 endpoint 2FA dikecualikan dari redirect global supaya kode yang
salah tetap menampilkan pesan pada form, bukan me-reload dan menghapus state.

Pengelolaan pada `/keamanan` juga lengkap:

- regenerate backup codes dan disable/re-enrol tersedia dari layar aktif;
- kedua endpoint sensitif ditahan server-side sampai TOTP/backup-code step-up
  yang terikat ke token sesi aktif dan berlaku lima menit;
- akun credential tetap harus mengonfirmasi password sesuai aturan Better
  Auth; akun Google-only tidak diminta password yang memang tidak ada;
- disable merotasi sesi, dan single-session hook mengakhiri sesi lain;
- enrolment, login factor, step-up, regenerasi, disable, dan challenge Google
  ditulis ke `audit_logs` tanpa kode atau secret.

`e2e/two-factor-security.spec.ts` melewati alur nyata: enrol TOTP, bypass
management ditolak 403, wrong-code tetap di form, step-up, regenerasi, login
dengan backup code, lalu disable sebagai akun tanpa credential. Regression
OTP-email → QR dan enrolment credential juga tetap hijau.

Satu validasi yang tetap manual setelah deploy: callback Google sungguhan
memerlukan provider Google eksternal. Logout penuh, masuk dengan Google pada
akun enrol, dan pastikan `/two-factor-challenge` tampil sebelum dashboard.

---

## Pencairan refund otomatis + refund pembelian agen (selesai 2026-08-29)

Pekerjaan audit nomor 3 dan 4 sudah selesai. Saldo refund jamaah tetap tersedia
di `/pilgrim/transactions`, dan pembelian agen untuk dirinya sendiri sekarang
dapat direfund ke wallet khusus pada tab **Beli Produk** di `/agent`. Operator
owner/admin memantau keduanya melalui **Refund & Saldo** di
`/dashboard/refunds`.

### Aturan uang yang sekarang berlaku

- Refund self-purchase agen masuk ke append-only
  `agent_refund_balance_entries`, bukan ledger komisi. Agen tidak mendapat
  komisi dari pembeliannya sendiri dan refund tidak dapat menciptakan atau
  memulihkan komisi.
- Satu payout punya tepat satu penerima: `PILGRIM` atau `AGENT`. Identitas
  penerima selalu diturunkan dari sesi Better Auth, tidak pernah dipercaya dari
  request. Pengajuan tetap wajib 2FA.
- Nomor rekening/e-wallet disimpan terenkripsi menggunakan
  `KYC_ENCRYPTION_KEY`; API/UI hanya mengembalikan empat digit terakhir.
- Idempotency key, advisory lock per penerima, partial unique index, balance
  trigger, dan deferred ledger trigger mencegah double reservation dan double
  debit walau request bersamaan atau webhook dikirim ulang.
- Status gateway adalah
  `REQUESTED -> PROCESSING -> PAID | FAILED`, dengan `PAID -> REVERSED` bila
  Xendit membalik transfer. Withdrawal dan reversal ledger selalu berada dalam
  transaksi yang sama dengan statusnya dan masing-masing hanya dapat ditulis
  sekali.

### Gateway dan payout tunai

- Tanpa `XENDIT_SECRET_KEY`, wallet API mengumumkan capability non-tunai
  sebagai tidak tersedia, UI jamaah/agen hanya menawarkan **Tunai**, dan
  backend menolak request bank/e-wallet sebelum menyimpan atau mereservasi
  saldo. Mengisi key yang valid mengaktifkan pilihan otomatis tanpa perubahan
  kode atau feature flag tambahan.
- Transfer bank/e-wallet dikirim oleh worker melalui Xendit Payouts v3 dengan
  UUID request sebagai reference serta idempotency key yang stabil. Tidak ada
  network call saat transaksi database terbuka.
- Webhook `POST /webhooks/xendit/payout` adalah wake-up signal saja. Server
  selalu mengambil ulang status lewat API Xendit terautentikasi sebelum
  mengubah ledger. Poller 30 detik merekonsiliasi webhook yang hilang dan
  timeout yang hasilnya ambigu.
- Tunai tetap manual. Owner/admin harus mengunggah bukti PDF/JPG/PNG (maksimum
  10 MB) sebelum `MARK_PAID`; file disimpan dengan nama acak dan mode `0640`.
  Volume production `uploads_data` membuat bukti dan dokumen lama tetap ada
  saat image/container diganti.
- Resolver upload memakai role organisasi sesungguhnya, bukan display name;
  hanya owner/admin yang dapat memasang bukti dan menyelesaikan payout tunai.

### Konfigurasi produksi yang masih harus dilakukan saat deploy

- Kunci `XENDIT_SECRET_KEY` harus mempunyai izin **MONEY-OUT**.
- Daftarkan callback PAYOUT:
  `https://tawafiqhub.id/webhooks/xendit/payout` dengan verification token yang
  sama pada `XENDIT_WEBHOOK_TOKEN`.
- Pertahankan nilai `KYC_ENCRYPTION_KEY` yang sudah dipakai; kunci itu sekarang
  juga membuka tujuan payout terenkripsi. Migration 120 dan `cmd/rotatekyc`
  sekarang merotasi KYC serta tujuan payout, tetapi wajib dilakukan dalam
  maintenance window dengan API/worker berhenti; lihat `DEPLOY.md` section 12c.
- `XENDIT_WEBHOOK_ALLOWED_IPS` masih menunggu rentang resmi dari support
  Xendit. Token + API re-fetch sudah aktif, tetapi allowlist tetap perlu
  dilengkapi.
- Refund payout sudah dipush dan dideploy melalui commit `26b1e08` dan
  fallback cash-only melalui `5301afe`.

Validasi yang sudah lulus mencakup migration 119 up/down/up, Buf lint/generate,
seluruh Go test/vet/build, frontend typecheck/lint/build, integration test
PostgreSQL untuk refund agen, reservation concurrency, payout terenkripsi,
webhook replay dan reversal, serta Playwright nyata untuk pengajuan jamaah,
wallet refund agen, dan payout tunai sampai PAID dengan tepat satu debit.

### Drawer Tambah Kloter (diperbaiki 2026-08-29)

Drawer kanan **Tambah Kloter** pada `/dashboard/kloter` sebelumnya memakai
`z-index: 30`, lebih rendah dari sticky topbar dashboard (`40`). Overlay kini
berada pada layer modal `70`, di atas topbar dan mobile navigation drawer.
Regression test memeriksa `elementFromPoint` tepat di area overlap atas pada
desktop 1280x800 dan mobile 390x844; judul serta tombol tutup juga wajib tetap
terlihat. Kedua viewport sudah diverifikasi lewat screenshot Playwright.

## Rotasi kunci mencakup tujuan payout (selesai lokal 2026-08-29)

Migration 120 menambahkan `destination_key_fingerprint` pada payout non-tunai.
Tulisan baru menyimpan ciphertext dan fingerprint kunci dalam satu INSERT;
startup API/worker sekarang melaporkan fingerprint KYC serta payout dan memberi
error keras bila data lama mengharapkan kunci berbeda atau belum memiliki
stamp.

`cmd/rotatekyc` sekarang merotasi dua kumpulan data: `kyc_records` dan tujuan
rekening/e-wallet di `pilgrim_refund_payout_requests`. Jalur khusus trigger
hanya membolehkan koneksi administratif mengganti ciphertext + fingerprint;
nominal, penerima, metode, status, dan seluruh bukti transaksi harus identik.
Role production `safrat_app` tidak dapat membuka jalur tersebut.

Rotasi harus dilakukan dalam maintenance window: stop API + worker, jalankan
tool sampai `still_on_old_key=0` dan `legacy_unstamped_payouts=0`, ganti env ke
kunci baru, lalu restart. Kunci lama tetap disimpan sampai backup lama keluar
dari masa retensi. Prosedur lengkap ada di `DEPLOY.md` section 12c.

Validasi: migration 120 up/down/up; full Go test/vet/build; integration test
PostgreSQL membuktikan perubahan biasa ditolak, ciphertext + fingerprint
berpindah bersama, hasil tetap terbaca dengan kunci baru, dan rerun idempotent.

## Fraud control + batas percobaan checkout (selesai lokal 2026-08-29)

Migration 121 menambahkan `checkout_channel`, `risk_level`, dan `risk_reason`
pada order. Kebijakan hanya berlaku pada gateway checkout (`XENDIT`); transaksi
manual tunai/transfer tidak ikut dihitung.

Aturan per pembeli dalam rolling window satu jam:

- replay dengan idempotency key yang sama mengembalikan order lama dan tidak
  menambah attempt;
- attempt baru 1–3 normal;
- attempt baru 4–5 dibuat dengan `risk_level=REVIEW`;
- attempt baru ke-6 ditolak dengan `ResourceExhausted` dan penolakan ditulis ke
  audit log;
- pembeli dengan order `HELD` yang belum diselesaikan tidak dapat membuka
  gateway checkout baru.

Semua keputusan count/insert diserialkan dengan PostgreSQL advisory lock per
pembeli, jadi request bersamaan tidak dapat melewati batas. Trigger kedua
mencegah order `REVIEW` berpindah langsung `PENDING -> PAID`, bahkan jika caller
melewati service. Pembayaran dengan nominal tepat tetap masuk `HELD` dan tidak
masuk revenue/komisi payable sampai owner/admin menerima atau menolaknya.

Dashboard order travel dan antrean transaksi platform menampilkan alasan risk.
Form jamaah/agen menerjemahkan penolakan menjadi pesan Bahasa Indonesia yang
bisa ditindak. Satu bug lama yang ikut ditemukan: `SellPackageDialog` tidak
mengirim idempotency key; sekarang satu key dibuat per dialog dan dipertahankan
saat retry yang hasilnya tidak pasti.

Validasi: migration 121 up/down/up, integration test 12 request bersamaan hanya
menghasilkan 5 order dan 2 `REVIEW`, replay tidak mengonsumsi attempt, pembayaran
tepat pada order berisiko menjadi `HELD`, direct database settlement ditolak,
dan review queue platform melihatnya. Seluruh service integration test, Go
test/vet/build, Buf lint/generate, frontend typecheck/lint/build lulus.

---

## Pengiriman perlengkapan fisik (selesai 2026-08-29)

Perlengkapan dijual dan tidak ada tempat mencatat penyerahannya. Order EQUIPMENT
yang sudah dibayar membuka fulfilment seperti yang lain, lalu berakhir di salah
satu dari dua keadaan yang sama-sama salah:

- jalur cepat tidak menemukan routing supplier → **PENDING selamanya**;
- sweep menandainya **NEEDS_REVIEW** dengan *"Produk belum punya routing supplier
  aktif"*.

Yang kedua lebih buruk. Ia mengarsipkan paket sebagai cacat routing yang tidak
akan pernah bisa diperbaiki — perlengkapan memang tidak punya supplier — jadi
antrean yang ada untuk dikosongkan justru penuh oleh hal yang tak bisa
ditindaklanjuti, dan kegagalan routing yang asli tenggelam di dalamnya.

### Yang menahannya sekarang, semuanya di database

- paket kurir tidak bisa ditandai terkirim tanpa kurir, resi, alamat, dan
  penerima; paket yang diambil sendiri tidak butuh satu pun dari itu;
- DELIVERED wajib punya nama penerima, staf yang mencatat, dan waktu — penyerahan
  adalah bukti, bukan status yang diklik;
- **tujuan membeku setelah paket berangkat.** Mengoreksi alamat hasil telepon itu
  pekerjaan biasa; menulis ulang tujuan paket yang sudah jalan bukan, dan tidak
  akan ada jejak bahwa ia pernah berbeda;
- serah terima tidak bisa dicatat dua kali — penulisan kedua akan menimpa siapa
  yang sebenarnya menerima.

`kind` (SUPPLIER/SHIPMENT) ditentukan dari kategori produk, **bukan** disimpulkan
dari apakah `supplier_id` kebetulan NULL. Penyimpulan itulah yang dulu memasukkan
paket ke antrean supplier.

**PICKUP adalah metode kelas satu**, bukan kasus pinggiran. Jamaah mengambil
koper di kantor itu normal di sini, dan memaksakan alamat berarti staf mengarang.

Layarnya di `/dashboard/shipments`, menu **Pengiriman**.

### Catatan aksesibilitas

Petunjuk di dalam `<label>` yang membungkus input ikut masuk ke nama aksesibel
field. "Diterima oleh" terbaca sebagai "Diterima oleh Nama orang yang menerima
barang". Sekarang dilampirkan lewat `aria-describedby`. Pola ini ada juga di
komponen lain — layak disisir kalau ada waktu.

### Belum dikerjakan

Bukti foto/scan tanda terima belum disimpan. Penyimpanan S3 sudah ada dan
dipakai storefront, jadi ini pekerjaan yang bisa disambung, bukan pondasi baru.

---

## Antrean "Perlu ditinjau" akhirnya bisa dikerjakan (2026-08-29)

Supplier di pasar ini menjawab dengan bentuk yang berubah tanpa pemberitahuan.
Kalau tidak ada aturan yang bisa membacanya, sistem **tidak menebak** — ia
menandai `NEEDS_REVIEW` dan menunggu manusia. Itu naluri yang benar: menebak
"terkirim" berarti menahan uang jamaah tanpa mengirim apa pun, menebak "gagal"
berarti mengirim dua kali dan menanggung biayanya.

Masalahnya, **manusia itu tidak punya tombol**. Panel admin menampilkan
antreannya lengkap dengan jawaban supplier yang tak terbaca, lalu berhenti.
`ResolveManually` ada di service dan tidak dipanggil dari mana pun — tanpa RPC,
tanpa handler, tanpa UI. Jadi transaksi yang hasilnya tak bisa ditentukan duduk
di sana permanen, uang sudah diambil, dan satu-satunya penyelesaian adalah
membuka database.

### Yang sekarang berlaku

- Owner/admin platform menyelesaikan lewat tombol **Tinjau** di tab Transaksi.
- Dua hasil saja: **sudah sampai** atau **gagal**. Tidak ada di antaranya —
  "mungkin beres" itu keadaan yang sedang diselesaikan, bukan jawabannya.
- **FAILED mengembalikan uang.** Tanpa itu ini lebih buruk daripada
  menggantung: catatannya akan berkata perkaranya selesai sementara jamaah
  tetap kehilangan uangnya.
- Alasan **wajib** — keputusan ini tidak dikonfirmasi apa pun di luar sistem,
  jadi alasannya adalah seluruh jejak pertanggungjawabannya.

### Koreksi yang ditemukan tes

Saya sempat menulis bahwa penyelesaian menolak percobaan kedua, dan karena itu
ia yang mencegah refund ganda. **Salah.** Penyelesaian ulang sengaja diizinkan
supaya keputusan keliru bisa dikoreksi — kalau tidak, operator yang salah klik
terjebak memegang uang jamaah tanpa jalan sah mengembalikannya.

Yang benar-benar mencegah pembayaran ganda adalah **refund-nya sendiri**: satu
per order di level database, dipanggil dengan kunci yang diturunkan dari id
order, bukan acak.

### Refund tidak diduplikasi

`RefundOrderForOperator` adalah jalur refund yang sudah ada dengan operator
dikirim sebagai argumen alih-alih diambil dari sesi — admin platform bertindak
atas order milik tenant yang bukan keanggotaannya. Operatornya dibaca **dari
order**, jadi tidak bisa diarahkan ke tenant yang salah.

### `build:verify`

`pnpm build` biasa menimpa `.next` yang sedang dipakai dev server, dan
kegagalannya muncul sebagai `fixture sign-in failed (500)` — persis seperti bug
autentikasi. Saya menabraknya **empat kali** meski sudah mendokumentasikannya.
Sekarang ada `pnpm --filter @hajj-saas/web build:verify` yang membangun ke
`distDir` sendiri. Pakai itu untuk verifikasi.

---

## Bukti foto, rekonsiliasi transfer, dan aria-describedby (2026-08-29)

### Bukti foto serah terima

Penyerahan dulu dibuktikan dengan nama yang diketik di kotak — catatan yang
nyata, dan jenis yang paling lemah: tidak ada yang membedakan nama yang ditulis
karena jamaah benar-benar menandatangani dari nama yang ditulis karena antrean
perlu dibersihkan.

**Fotonya disimpan privat**, bukan di jalur aset publik yang dipakai gambar
storefront. Struk penerimaan memperlihatkan nama orang, tanda tangannya, dan
sering pintu rumahnya. Dibaca lewat tautan bertanda tangan berumur lima menit,
diambil saat dibutuhkan — daftar pengiriman tidak pernah membawa tautan ke
pintu rumah orang.

Kuncinya **diturunkan** dari operator dan order, bukan dikirim pemanggil, dan
diverifikasi ke penyimpanan sebelum dicatat. Kunci yang menunjuk objek tak ada
akan terbaca sebagai bukti atas sesuatu yang tidak pernah terjadi — lebih buruk
daripada tanpa foto, karena barisnya mengaku terdokumentasi.

Kalau penyimpanan belum dikonfigurasi, fotonya tidak ditawarkan dan penyerahan
tetap bisa dicatat. Mencatat nama penerima tetap berharga sendirian.

### Pencocokan transfer bank

Kelas yang sama dengan antrean tinjau kemarin: **mesinnya ada, pemicunya
tidak.** Setiap tagihan sudah membawa nominal unik sampai rupiah, dan
`FindPayableByAmount` sudah ada untuk mencocokkannya — tanpa satu pun pemanggil.
Jadi mekanismenya dibangun, disimpan, dan tak terjangkau; mengonfirmasi transfer
berarti membuka database.

Sekarang admin mengetik angkanya dari mutasi rekening, di tab **Transfer**.
**Nominal yang dibulatkan tidak cocok** — itu benar, bukan kelonggaran yang
perlu ditambahkan: salah mengkredit travel jauh lebih buruk daripada meminta
orang membaca ulang angkanya.

`FindPayableByAmount` sekarang mengembalikan nama travel dalam query yang sama.
Mengambilnya setelah pelunasan tidak akan menemukan apa pun — invoice-nya sudah
tidak PENDING lagi. Itu bug yang saya tulis dan tangkap sebelum terkirim.

**Belum otomatis penuh.** Otomatisasi sejati butuh feed mutasi dari bank; ini
menghapus langkah SQL-nya, bukan langkah manusianya.

### aria-describedby

Petunjuk di dalam `<label>` yang membungkus input ikut masuk ke nama aksesibel.
Diperbaiki di `MovementFormDialog`, `KloterFormDialog`, `CatalogueTab`, dan
`ShipmentsDashboard`.

**`PilgrimFormDialog` masalahnya berbeda dan lebih besar**: labelnya `<span>`
di dalam `<div>`, jadi tidak ada asosiasi terprogram sama sekali antara label
dan input. Dicatat, bukan ditambal setengah — perbaikannya menyentuh banyak
field sekaligus.

### `.next-verify` diabaikan eslint

`build:verify` membuat lint gagal dengan 86 error di output terkompilasi.

---

## Label jamaah & rekonsiliasi bank otomatis (2026-08-29)

### PilgrimFormDialog: label kini terhubung

Labelnya `<span>` di dalam `<div>` — tidak ada yang menghubungkannya ke kontrol.
Pembaca layar mengumumkan **tiga belas kotak tanpa nama**, dan petunjuk maupun
pesan kesalahan tidak pernah dibacakan bersama field-nya. Ini form terpanjang di
aplikasi, dan tempat seluruh catatan seorang jamaah diketik.

`fieldKey` sudah unik per field, jadi id-nya diturunkan dari situ — tidak ada
prop baru yang harus dijaga selaras. Kesalahan validasi diberi `role="alert"`
supaya diumumkan saat muncul, bukan baru ditemukan oleh orang yang kebetulan
menavigasi kembali.

### Rekonsiliasi transfer bank

Tagihan sudah membawa nominal unik sampai rupiah, dan pencocoknya sudah ada;
yang hilang adalah **yang memberinya makan**. Sekarang poller atau scraper
mengirim mutasi ke `POST /webhooks/bank-feed` dan pencocokan berjalan sendiri.

**Feed itu masukan tidak tepercaya** — API bank di hari baik, scraper yang
membaca HTML orang lain di hari biasa. Jadi:

- **Ditandatangani HMAC atas isi badan**, bukan token di header. Token hanya
  membuktikan pengirim tahu tokennya; tanda tangan juga membuktikan badannya
  tidak diubah di tengah jalan — dan badan itu melunasi tagihan.
- **Tanpa `BANK_FEED_SECRET`, endpoint menolak semuanya.** Endpoint yang
  memberikan akses berbayar tidak boleh terbuka karena variabel terlupa.
- **Kredit dicatat sebelum dicocokkan**, dan tetap dicatat meski tidak cocok.
  Kredit tak cocok adalah baris terpenting di tabel itu: uang masuk yang tidak
  diakui apa pun.
- **Hanya nominal persis.** Pendekatan kira-kira berarti menebak travel mana
  yang membayar, dan salah mengkredit jauh lebih buruk daripada menyisakan
  kredit untuk ditempatkan manusia.

**Urutan penyelesaian**: mutasi dipindahkan lebih dulu, karena baris itulah yang
memegang indeks unik `matched_invoice_id` — itu yang membuat satu kredit mustahil
melunasi dua kali, dan yang membuat pencocok otomatis dan manusia yang mengklik
aman berjalan bersamaan. Keduanya satu transaksi.

### Jalur manual tetap ada, dan kini per tiket

Sesuai permintaan owner: admin melampirkan kredit yang **benar-benar tercatat**
ke tagihan tertentu — untuk biaya admin bank yang memotong, nominal yang salah
terbaca, atau scraper yang rusak. Alasan wajib. Kredit yang bukan pembayaran
langganan ditandai **IGNORED, bukan dihapus**: uangnya tetap masuk, dan catatan
itu harus hidup lebih lama daripada keputusan tentangnya.

### Yang perlu disiapkan

`BANK_FEED_SECRET` di `.env.prod`. Poller/scraper-nya sendiri belum ditulis —
kontraknya ada di `apps/api/internal/handler/bank_feed.go`, dan formatnya JSON
sederhana dengan header `X-Signature`.

---

## Poller bank & penagihan perpanjangan (2026-08-30)

### `cmd/bankpoller`

Endpoint feed sudah ada dan tidak ada yang mengirim ke sana, jadi otomatisasinya
**siap tapi belum bekerja**. Ini pengirimnya.

Proses terpisah, bukan task worker: pembaca mutasi adalah bagian paling rapuh di
sistem ini, dan mungkin perlu berjalan di jaringan atau mesin lain. Di luar
proses API, kegagalannya tidak menjatuhkan API.

Sumbernya antarmuka dua method dengan satu implementasi nyata: **CSV**. Bukan
placeholder — setiap bank Indonesia bisa mengekspor mutasi, jadi ini bekerja
hari ini tanpa akses API. API bank sungguhan tinggal mengisi `bankfeed.Source`.

**Dibuktikan ujung ke ujung** terhadap API yang berjalan: satu kredit cocok
otomatis, debit dilewati, satu kredit tersisa di antrean, tagihan jadi PAID dan
operatornya **benar-benar mendapat paket GROWTH**. Impor ulang: nol baris baru.

Dua bug tertangkap saat membangun:
- pemisah angka — `1,500,000` terbaca 1500. Rp1,5 juta jadi Rp1.500, tidak cocok
  dengan tagihan mana pun, terbaca persis seperti pelanggan yang tidak bayar;
- laporan mencetak `belum_cocok=-1` pada impor ulang. Angka negatif di log yang
  seharusnya ditindaklanjuti lebih buruk daripada tidak ada angka.

**Batasan yang didokumentasikan, bukan disembunyikan**: tanpa kolom referensi,
id diturunkan dari isi baris, jadi dua transfer identik di hari yang sama
dianggap satu. Mengurangi hitungan, bukan melunasi dua kali.

### Tagihan perpanjangan akhirnya diterbitkan

Sweep dulu hanya **mengedaluwarsakan** tagihan dan menandai langganan lewat
masa — tidak pernah menagih periode berikutnya. Langganan berhenti begitu saja
dan pendapatan berakhir tanpa ada yang memutuskan.

Diterbitkan **seminggu di muka**, supaya transfer sempat datang dan dicocokkan
sebelum akses habis. Satu per operator, dijaga di query bukan di Go. Langganan
yang sudah dibatalkan dilewati.

### Kredit menganggur kini masuk log

Sebelumnya hanya terlihat oleh yang membuka tab admin. Kredit yang duduk dua
hari berarti ada travel yang percaya sudah membayar sementara sistem tidak
setuju — itu layak WARN, tempat alerting membaca.

### Cacat yang saya buat sendiri di migrasi 122

Alarm fulfilment macet **sudah ada** — catatan saya kemarin yang bilang belum
ada itu salah. Tapi query-nya tidak membedakan jenis, dan SHIPMENT baru ada
minggu lalu.

Supplier menjawab dalam hitungan detik, jadi diam satu jam adalah cacat. Kurir
butuh berhari-hari — ambang yang sama akan memicu alarm untuk **setiap paket
yang sedang di jalan, setiap sweep, sampai tiba**. Itu akan membanjiri alarm
dalam hitungan hari dan melatih semua orang mengabaikannya, sehingga merusak
gunanya bagi fulfilment digital yang jadi alasan ia dibangun.

Paket sekarang diberi **dua minggu** — saat sebuah paket berhenti "di jalan" dan
mulai "hilang".

---

## Remediasi environment + rotasi KYC production (2026-08-30)

Audit menemukan delapan salinan `.env.prod` lama/salah nama di checkout VPS;
satu nama file bahkan membawa nilai KYC key. Semuanya sudah dihapus dengan
guard yang mempertahankan tepat `.env.prod` aktif, mode `0600`. Pola
`.env.prod*` sekarang diabaikan Git, dan semua environment lokal/production
dikeluarkan dari Docker build context. Sekalian, artefak `.next-verify`, pnpm
store, dan workspace referensi dikeluarkan sehingga context API turun dari
lebih dari 1 GB menjadi sekitar 27 MB.

Runbook rotasi sebelumnya tidak bisa dijalankan sungguhan: image tidak membawa
binary `rotatekyc`, command mengharuskan `DATABASE_URL` sementara production
memakai variabel `PG*`, dan maintenance shell dapat diam-diam memilih cache tag
`latest` lama. Ketiganya diperbaiki. Maintenance sekarang memakai binary
`/rotatekyc`, koneksi libpq `PG*`, owner database secara eksplisit, `--no-deps`,
dan immutable tag dari `git rev-parse HEAD`.

Sebelum rotasi, dump production dibuat terenkripsi di laptop dengan private key
yang tidak pernah masuk VPS. Restore rehearsal membuktikan checksum, migration
122, dan row count. Private key kemudian dikunci AES-256 dengan passphrase acak
yang disimpan sebagai Generic Password di macOS Keychain; arsip dibuka ulang
dengan key terkunci dan checksum kembali cocok.

Production berotasi dari fingerprint `3b5847c7` ke `da3a6362`. Saat maintenance
tidak ada KYC atau tujuan payout terenkripsi, tetapi rotator tetap dijalankan
dan melaporkan `still_on_old_key=0` serta `legacy_unstamped_payouts=0` sebelum
env diganti secara atomik. Audit akhir: hanya satu file `.env.prod`, tidak ada
temp rotation, fingerprint env `da3a6362`, API/ready/web semuanya HTTP 200.

**Yang masih harus dilakukan owner:** salin `backup-key.locked.pem` dan arsip
pre-rotasi ke media kedua/off-device (misalnya iCloud Drive atau flash disk),
uji bahwa salinannya ada, lalu hapus `backup-key.pem` asli yang belum terkunci.
Recurring R2 backup juga belum dipasang di VPS; backup ini satu kali, bukan
pengganti cron off-site. `BANK_FEED_SECRET` masih belum diset dan tetap
menghasilkan warning Compose.

---

## Redesign landing page utama (2026-08-30)

Landing page `tawafiqhub.id` sudah dibangun ulang agar mencerminkan produk yang
sekarang, bukan hanya rooming dan manifest bus. Information architecture baru
menjelaskan empat lapisan utama: data jamaah, keberangkatan, operasional
lapangan, serta bisnis travel. Pengalaman operator, storefront travel, jamaah,
tour leader, agen, dan kontrol keamanan juga tampil sebagai satu ekosistem.

Hero memakai foto operasional khusus TawafiqHub yang dihasilkan untuk proyek
ini dan dioptimalkan menjadi WebP 69 KB. Bagian pendukung memakai aset editorial
yang sudah ada di repository. Mock dashboard, angka ROI tanpa sumber, logo
regulator palsu, dan tiga testimoni yang belum dapat diverifikasi sudah
dihapus. Harga tetap memakai angka yang sebelumnya sudah disepakati, dengan
catatan eksplisit bahwa domain, layanan pihak ketiga, dan integrasi khusus tidak
termasuk.

Halaman sekarang server-rendered dengan metadata khusus, Open Graph, dan
structured data `SoftwareApplication`. Mode terang/gelap, menu mobile, Escape
untuk menutup drawer, reduced motion, breakpoint mobile, dan CTA autentikasi
tetap dipertahankan. Bundle route utama turun dari sekitar 14,4 KB menjadi
9,85 KB. Typecheck, ESLint, production build, dan respons HTML production
preview semuanya lulus.

**Belum dilakukan:** visual inspection otomatis desktop/mobile dan Lighthouse
karena browser terintegrasi tidak tersedia pada sesi ini. Production belum
diubah; redesign baru ada di repository lokal sampai commit ini didorong dan
dideploy.

---

## Paket hilang akhirnya punya jalan keluar (2026-08-30)

Alarm dua minggu yang saya tambahkan kemarin memberi tahu bahwa paket tidak
sampai, **dan tidak menawarkan apa pun untuk menutupnya**. Tidak bisa ditandai
diserahkan karena memang belum; tidak bisa ditandai hilang sama sekali. Uang
sudah diambil, alarm berulang tiap sweep, tidak ada yang bisa dilakukan.

Itu cacat yang sama persis dengan yang saya perbaiki untuk fulfilment supplier
dua hari sebelumnya — **diperkenalkan lagi dengan menambah alarm tanpa pintu
keluar**. Pola yang layak diingat: setiap alarm baru harus datang bersama cara
menutupnya.

### Yang berlaku sekarang

- Menyatakan paket hilang **mengembalikan uang**. Tanpa itu ini lebih buruk
  daripada alarm yang dibungkamnya: catatannya berkata perkara selesai
  sementara jamaah tetap kehilangan uangnya.
- **Fulfilment dipindahkan lebih dulu**, dan update itu bersyarat pada paket
  masih terbuka — itu yang mencegah dua klik jadi dua refund.
- **Paket yang sudah diserahkan tidak bisa dinyatakan hilang.** Serah terima
  yang tercatat adalah bukti seseorang menandatanganinya, dan bukti tidak boleh
  bisa dihapus oleh klik berikutnya.
- Tombolnya baru muncul setelah paket benar-benar dikirim. Sebelum itu ia bukan
  hilang, melainkan belum dikirim — dan jalan keluarnya adalah mengirimnya.

### Masih terbuka

Notifikasi ke travel saat pembayaran diakui. Dengan pencocokan otomatis, sebuah
tagihan bisa lunas tengah malam tanpa siapa pun tahu.

---

## Notifikasi pembayaran ke travel (2026-08-30)

Travel mentransfer lalu menunggu **tanpa sinyal apa pun**. Pencocokan otomatis
justru memperburuknya: sebuah tagihan kini bisa lunas jam tiga pagi tanpa ada
yang menyaksikan, dan diam setelah pembayaran terbaca seperti pembayaran yang
tidak sampai.

Pesan yang sama di **setiap jalur** — cocok otomatis, kredit yang dilampirkan,
atau nominal yang diketik dari mutasi. Mengetahui apakah kamu diberi tahu
berdasarkan jalur mana yang kebetulan melunasi bukan rancangan, itu kebetulan.

Nominalnya ikut di pesan. "Pembayaran diterima" tanpa angka tetap membuat
pembacanya harus mengecek sendiri.

### Yang dijaga

- **Notifikasi tidak pernah menghalangi pelunasan.** Uang sudah berpindah dan
  akses sudah diberikan; membatalkannya karena push gagal terkirim adalah
  pertukaran yang salah.
- Tanpa Firebase, ia no-op. Antarmukanya ada supaya pelunasan tidak bergantung
  padanya sama sekali.
- **Pointer nil di dalam antarmuka non-nil** adalah jebakan Go yang persis
  diundang bentuk ini. Method `FirebasePusher` memang memeriksa `p == nil` —
  saya pastikan, bukan asumsikan.
- **Subjek tagihan dibaca sebelum melunasi.** Sesudah PAID ia tidak lagi
  pending dan lookup-nya menemukan nol — kesalahan yang saya buat di jalur
  konfirmasi-nominal minggu lalu. Sekarang ada tes yang gagal kalau urutannya
  ditukar.

---

## Form publik: sisi jamaah (2026-08-30)

**Koreksi dulu.** Sapuan saya sebelumnya menyebut tujuh komponen punya label
tak terhubung. Itu terlalu kasar: form-form ini membungkus input di dalam
`<label>`, jadi asosiasinya sudah benar. Pola yang benar-benar rusak adalah
`<span>` di dalam `<div>` tanpa elemen `<label>` sama sekali — dan itu hanya
`PilgrimFormDialog`, yang sudah diperbaiki.

Yang sapuan itu munculkan justru celah lain, dan lebih langsung terasa
konsumen. Form pendaftaran publik diisi jamaah, biasanya di ponsel, sering oleh
orang yang tidak nyaman dengan formulir — dan ia membuka **keyboard qwerty
penuh untuk nomor telepon**.

### Yang berubah

- `type="tel"` + `inputMode` → papan angka yang terbuka, bukan qwerty
- Email: `autocapitalize="none"`, `autocorrect="off"` — kapitalisasi otomatis
  adalah cara `Budi@` sampai ke server
- `autoComplete` di setiap field yang bisa diisi otomatis → form panjang jadi
  beberapa ketukan
- Nomor paspor dikapitalkan saat diketik, bukan ditolak sesudahnya
- Tanggal lahir dibatasi sampai hari ini. Ulang tahun di masa depan itu salah
  ketik, dan pemilihnya bisa menolaknya di tempat kesalahan dibuat

### Yang ternyata lebih baik dari rencana saya

Menambahkan `required` membuat **browser sendiri** yang menolak pengiriman dan
memfokuskan field yang kosong. Itu lebih baik daripada pesan kustom yang saya
hampir tulis tesnya: satu kalimat di atas form yang sudah di-scroll lewat tidak
memberi tahu field mana yang salah.

Tes saya semula memeriksa jalur kustom itu dan **salah premis**. Sekarang ia
memeriksa perilaku yang sebenarnya.

Pesan kustom tetap ada untuk kegagalan server — dan itu diumumkan serta
mengambil fokus. Di ponsel tombol kirim ada di bawah form panjang, jadi pesan
yang dirender diam-diam di atasnya tidak akan terlihat. Layar berhasil juga
memindahkan fokus ke judulnya, karena ia mengganti seluruh form dan tanpa itu
fokus tertinggal di tombol yang sudah tidak ada.

`ApplyAsAgentForm` dapat perlakuan sama — ia juga publik.
