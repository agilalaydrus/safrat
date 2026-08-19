"use client";

import { useEffect, useState } from "react";
import { IconMailForward, IconTrash, IconX } from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";
import { RoleGate } from "@/components/auth/RoleGate";

const ROLE_LABEL: Record<string, string> = { owner: "Pemilik", admin: "Admin", member: "Anggota" };

type Member = { id: string; userId: string; role: string; createdAt: Date; user: { id: string; name: string; email: string } };
type Invitation = { id: string; email: string; role: string | string[]; status: string };

export default function TeamPanel() {
  const { data: session } = authClient.useSession();
  const [members, setMembers] = useState<Member[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [working, setWorking] = useState("");
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("member");
  const [inviting, setInviting] = useState(false);

  const refresh = () => {
    setLoading(true);
    Promise.all([
      authClient.organization.listMembers({ query: {} }),
      authClient.organization.listInvitations({ query: {} }),
    ]).then(([membersResult, invitationsResult]) => {
      if (membersResult.data) setMembers(membersResult.data.members as Member[]);
      if (invitationsResult.data) setInvitations((invitationsResult.data as Invitation[]).filter((invite) => invite.status === "pending"));
    }).catch(() => setNotice("Gagal memuat anggota tim.")).finally(() => setLoading(false));
  };
  useEffect(refresh, []);

  const invite = async () => {
    if (!inviteEmail.trim()) return;
    setInviting(true);
    setNotice("");
    try {
      const result = await authClient.organization.inviteMember({ email: inviteEmail.trim(), role: inviteRole as "member" | "admin" | "owner" });
      if (result.error) { setNotice(result.error.message ?? "Gagal mengirim undangan."); return; }
      setInviteEmail("");
      setNotice(`Undangan terkirim ke ${inviteEmail.trim()}.`);
      refresh();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Gagal mengirim undangan.");
    } finally {
      setInviting(false);
    }
  };

  const changeRole = async (member: Member, role: string) => {
    setWorking(member.id);
    setNotice("");
    try {
      const result = await authClient.organization.updateMemberRole({ memberId: member.id, role: role as "member" | "admin" | "owner" });
      if (result.error) { setNotice(result.error.message ?? "Gagal mengubah peran."); return; }
      refresh();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Gagal mengubah peran.");
    } finally {
      setWorking("");
    }
  };

  const removeMember = async (member: Member) => {
    setWorking(member.id);
    setNotice("");
    try {
      const result = await authClient.organization.removeMember({ memberIdOrEmail: member.user.email });
      if (result.error) { setNotice(result.error.message ?? "Gagal menghapus anggota."); return; }
      setNotice(`${member.user.name} dikeluarkan dari tim.`);
      refresh();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Gagal menghapus anggota.");
    } finally {
      setWorking("");
    }
  };

  const cancelInvitation = async (invitation: Invitation) => {
    setWorking(invitation.id);
    setNotice("");
    try {
      const result = await authClient.organization.cancelInvitation({ invitationId: invitation.id });
      if (result.error) { setNotice(result.error.message ?? "Gagal membatalkan undangan."); return; }
      refresh();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Gagal membatalkan undangan.");
    } finally {
      setWorking("");
    }
  };

  return <div style={{ display: "grid", gap: 20 }}>
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    <section style={card}>
      <h2 style={{ margin: 0 }}>Undang Anggota Baru</h2>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <input type="email" value={inviteEmail} onChange={(e) => setInviteEmail(e.target.value)} placeholder="email@contoh.com" style={{ ...input, flex: "1 1 220px" }} />
        <select value={inviteRole} onChange={(e) => setInviteRole(e.target.value)} style={{ ...input, width: "auto" }}>
          <option value="member">Anggota</option>
          <option value="admin">Admin</option>
        </select>
        <button disabled={inviting || !inviteEmail.trim()} onClick={invite} style={emerald}><IconMailForward size={18} />{inviting ? "Mengirim..." : "Undang"}</button>
      </div>
    </section>

    {!!invitations.length && <section style={card}>
      <h2 style={{ margin: 0 }}>Undangan Tertunda</h2>
      <div style={{ display: "grid", gap: 8 }}>
        {invitations.map((invitation) => <div key={invitation.id} style={row}>
          <span>{invitation.email} <span style={{ color: "var(--color-warm-400)", fontSize: 12 }}>({ROLE_LABEL[Array.isArray(invitation.role) ? invitation.role[0] ?? "member" : invitation.role] ?? invitation.role})</span></span>
          <button disabled={working === invitation.id} onClick={() => cancelInvitation(invitation)} style={ghostDanger}><IconX size={16} />Batalkan</button>
        </div>)}
      </div>
    </section>}

    <section style={card}>
      <h2 style={{ margin: 0 }}>Anggota Tim</h2>
      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <div style={{ display: "grid", gap: 8 }}>
        {members.map((member) => <div key={member.id} style={row}>
          <div><strong>{member.user.name}</strong><span style={{ display: "block", fontSize: 12, color: "var(--color-warm-400)" }}>{member.user.email}</span></div>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <select disabled={working === member.id || member.userId === session?.user?.id} value={member.role} onChange={(e) => changeRole(member, e.target.value)} style={{ ...input, minHeight: 36, width: "auto" }}>
              <option value="owner">Pemilik</option>
              <option value="admin">Admin</option>
              <option value="member">Anggota</option>
            </select>
            {member.userId !== session?.user?.id && <RoleGate require="owner"><button disabled={working === member.id} onClick={() => removeMember(member)} style={ghostDanger}><IconTrash size={16} /></button></RoleGate>}
          </div>
        </div>)}
      </div>}
    </section>
  </div>;
}

const card: React.CSSProperties = { display: "grid", gap: 12, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px", display: "inline-flex", gap: 8, alignItems: "center" };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, padding: 12, border: "1px solid var(--color-cream-300)", borderRadius: 8, background: "white", flexWrap: "wrap" };
const ghostDanger: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", padding: "0 10px", display: "inline-flex", gap: 6, alignItems: "center" };
