# Handoff Notes

> Working state + prioritized roadmap for the next agent. Point-in-time snapshot
> (2026-08-26). Verify against current code before trusting any file:line.

## Owner workflow preferences

- After verified implementation work, always create a local commit so progress
  is recorded. Never push or deploy unless the owner explicitly asks.
- After every commit, make sure the local development server is running again
  and verify the web endpoint so the owner can immediately inspect the result.
- Every handoff response must include: a concise summary, completed work,
  remaining/unverified work, recommendations, local commit, and server status.

## Continuation after this snapshot

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

**Still open from this:** an agent or muttawwif buying *for themselves* is not
possible. `orders.pilgrim_id` is `NOT NULL`, so the buyer must be a jamaah, and
the agent-to-agent referral chain (`agents.referred_by_agent_id`, which exists
and is unused for money) therefore never pays anyone. Supporting a non-jamaah
buyer is a schema change touching every order query — **needs the owner's
go-ahead before starting.**

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

**The addresses are deliberately left as placeholders, commented out.** I could
not verify Xendit's current egress ranges, and a wrong list at the edge blocks
every payment notification where the application-level guard cannot soften it.
Read them from the Xendit dashboard, paste into all three places, `nginx -t`,
then reload. Until then the application guard runs open and warns at startup,
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

#### Open — ordered

1. **Duplicate orders.** `CreateOrder` has no idempotency key and `orders` has
   no unique index preventing it. A double-click creates two orders and two
   Xendit invoices, and the pilgrim can pay both — charged twice for one
   intent. Same class as the three fixed above; fix it the same way, in the
   database.

2. **Fulfilment does not exist for anything but travel packages.** Product
   categories are `TRAVEL_PACKAGE`, `EQUIPMENT`, `ROAMING_DATA`, `PPOB_CREDIT`,
   but `applyPaidSideEffects` only acts on `TRAVEL_PACKAGE` (auto-kloter).
   - `EQUIPMENT` (physical): `orders` has no delivery status, address, tracking
     or handover proof. Safe only while handover is manual and in person.
   - `ROAMING_DATA` (digital): no voucher/eSIM issued or stored.
   - `PPOB_CREDIT` (digital): **no provider integration at all** — the pilgrim
     pays and no credit is ever sent. Consider disabling this category until it
     can be fulfilled.
   When PPOB is built, idempotency is critical: Xendit redelivers webhooks, and
   without a key on the fulfilment side one payment becomes two top-ups.

4. **Webhook does not verify the paid amount.** It reads only `id` and
   `status`. Xendit invoices are fixed-amount so practical risk is low, but
   comparing the amount is one line and is standard practice.

5. **No fraud/suspect handling.** No flag, no manual hold, no attempt limits.
   Card fraud detection sits with Xendit's hosted checkout, but there is no way
   for the operator to hold a suspicious order out of the totals.

6. **Receipts are per pilgrim, not per transaction.**
   `/dashboard/pilgrims/[id]/invoice` works well — order list, paid total,
   print/PDF, `@media print`. Missing: a per-transaction receipt with a
   referenceable invoice number, and any way for the *pilgrim* to see or print
   their own proof of payment. Today only the operator can.

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
