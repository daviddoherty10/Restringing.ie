package marketplace

import (
	"context"
	"time"

	"github.com/daviddoherty10/Restringing.ie/backend/internal/auth"
)

type TeamMember struct {
	UserID string `json:"user_id"`
	Phone  string `json:"phone_number"`
	Role   string `json:"role"`
}
type StaffInvitation struct {
	ID           string    `json:"id"`
	StringerID   string    `json:"stringer_id"`
	BusinessName string    `json:"business_name"`
	Phone        string    `json:"phone_number"`
	State        string    `json:"state"`
	ExpiresAt    time.Time `json:"expires_at"`
}
type Team struct {
	Members     []TeamMember      `json:"members"`
	Invitations []StaffInvitation `json:"invitations"`
}
type StaffInviteInput struct {
	Phone string `json:"phone_number"`
}

func (s *Store) Team(ctx context.Context, a Actor, id string) (Team, error) {
	out := Team{Members: []TeamMember{}, Invitations: []StaffInvitation{}}
	if e := authorize(ctx, s.Pool, a, id, true); e != nil {
		return out, e
	}
	rows, e := s.Pool.Query(ctx, `SELECT m.user_id::text,u.phone_number,m.role FROM stringer_members m JOIN users u ON u.id=m.user_id WHERE m.stringer_id=$1 ORDER BY m.role,m.user_id LIMIT 51`, id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var v TeamMember
		if e = rows.Scan(&v.UserID, &v.Phone, &v.Role); e != nil {
			rows.Close()
			return out, e
		}
		out.Members = append(out.Members, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	rows, e = s.Pool.Query(ctx, `SELECT i.id::text,i.stringer_id::text,s.name,i.phone_number,i.state,i.expires_at FROM staff_invitations i JOIN stringers s ON s.id=i.stringer_id WHERE i.stringer_id=$1 AND i.state='pending' AND i.expires_at>now() ORDER BY i.created_at DESC LIMIT 50`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v StaffInvitation
		if e = rows.Scan(&v.ID, &v.StringerID, &v.BusinessName, &v.Phone, &v.State, &v.ExpiresAt); e != nil {
			return out, e
		}
		out.Invitations = append(out.Invitations, v)
	}
	return out, rows.Err()
}
func (s *Store) InviteStaff(ctx context.Context, a Actor, id string, input StaffInviteInput) (ReadResult, error) {
	phone, e := auth.NormalizePhone(input.Phone)
	if e != nil {
		return ReadResult{}, invalid(map[string]string{"phone_number": "Use an international phone number."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return ReadResult{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return ReadResult{}, e
	}
	if e = authorize(ctx, tx, a, id, true); e != nil {
		return ReadResult{}, e
	}
	var count int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM team_events WHERE action='invite' AND stringer_id=$1 AND created_at>now()-interval '1 hour'`, id).Scan(&count); e != nil {
		return ReadResult{}, e
	}
	if count >= 20 {
		return ReadResult{}, problem(429, "invitation_limit", "Wait before inviting more staff.")
	}
	var member bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stringer_members m JOIN users u ON u.id=m.user_id WHERE m.stringer_id=$1 AND u.phone_number=$2)`, id, phone).Scan(&member); e != nil {
		return ReadResult{}, e
	}
	if member {
		return ReadResult{}, problem(409, "already_member", "This number already belongs to the team.")
	}
	if e = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM stringer_members WHERE stringer_id=$1)+(SELECT count(*) FROM staff_invitations WHERE stringer_id=$1 AND state='pending' AND expires_at>now())`, id).Scan(&count); e != nil {
		return ReadResult{}, e
	}
	if count >= 50 {
		return ReadResult{}, problem(409, "team_limit", "A business can have up to 50 members and pending invitations.")
	}
	var invitationID string
	e = tx.QueryRow(ctx, `INSERT INTO staff_invitations(stringer_id,phone_number,invited_by) VALUES($1,$2,$3) ON CONFLICT(stringer_id,phone_number) DO UPDATE SET invited_by=EXCLUDED.invited_by,state='pending',expires_at=now()+interval '7 days',created_at=now() RETURNING id::text`, id, phone, a.ID).Scan(&invitationID)
	if e != nil {
		return ReadResult{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO team_events(stringer_id,actor_id,action,subject) VALUES($1,$2,'invite',$3)`, id, a.ID, invitationID); e != nil {
		return ReadResult{}, e
	}
	return ReadResult{true}, tx.Commit(ctx)
}
func (s *Store) MyInvitations(ctx context.Context, a Actor) ([]StaffInvitation, error) {
	out := []StaffInvitation{}
	rows, e := s.Pool.Query(ctx, `SELECT i.id::text,i.stringer_id::text,s.name,i.phone_number,i.state,i.expires_at FROM staff_invitations i JOIN stringers s ON s.id=i.stringer_id JOIN users u ON u.phone_number=i.phone_number WHERE u.id=$1 AND u.status='active' AND u.phone_verified_at IS NOT NULL AND i.state='pending' AND i.expires_at>now() ORDER BY i.created_at DESC LIMIT 50`, a.ID)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v StaffInvitation
		if e = rows.Scan(&v.ID, &v.StringerID, &v.BusinessName, &v.Phone, &v.State, &v.ExpiresAt); e != nil {
			return out, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) AcceptInvitation(ctx context.Context, a Actor, id string) (ReadResult, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return ReadResult{}, e
	}
	defer tx.Rollback(ctx)
	var phone string
	if e = tx.QueryRow(ctx, `SELECT phone_number FROM users WHERE id=$1 AND status='active' AND phone_verified_at IS NOT NULL FOR SHARE`, a.ID).Scan(&phone); e != nil {
		return ReadResult{}, missing(e)
	}
	var shop string
	if e = tx.QueryRow(ctx, `SELECT stringer_id::text FROM staff_invitations WHERE id=$1 AND phone_number=$2`, id, phone).Scan(&shop); e != nil {
		return ReadResult{}, missing(e)
	}
	if _, e = lockStringer(ctx, tx, shop); e != nil {
		return ReadResult{}, e
	}
	var state string
	var live bool
	if e = tx.QueryRow(ctx, `SELECT state,expires_at>clock_timestamp() FROM staff_invitations WHERE id=$1 FOR UPDATE`, id).Scan(&state, &live); e != nil {
		return ReadResult{}, e
	}
	if state == "accepted" {
		return ReadResult{true}, tx.Commit(ctx)
	}
	if state != "pending" || !live {
		return ReadResult{}, problem(409, "invitation_expired", "This invitation has expired or was revoked.")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff') ON CONFLICT DO NOTHING`, shop, a.ID); e != nil {
		return ReadResult{}, e
	}
	if _, e = tx.Exec(ctx, `UPDATE staff_invitations SET state='accepted' WHERE id=$1`, id); e != nil {
		return ReadResult{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO team_events(stringer_id,actor_id,action,subject) VALUES($1,$2::uuid,'accept',$2::text)`, shop, a.ID); e != nil {
		return ReadResult{}, e
	}
	return ReadResult{true}, tx.Commit(ctx)
}
func (s *Store) RemoveStaff(ctx context.Context, a Actor, shop, id string, invitation bool) (ReadResult, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return ReadResult{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, shop); e != nil {
		return ReadResult{}, e
	}
	if e = authorize(ctx, tx, a, shop, true); e != nil {
		return ReadResult{}, e
	}
	action := "remove"
	if invitation {
		action = "revoke"
		_, e = tx.Exec(ctx, `UPDATE staff_invitations SET state='revoked' WHERE id=$1 AND stringer_id=$2 AND state='pending'`, id, shop)
	} else {
		var owner bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stringer_members WHERE stringer_id=$1 AND user_id=$2 AND role='owner')`, shop, id).Scan(&owner); e != nil {
			return ReadResult{}, e
		}
		if owner {
			return ReadResult{}, problem(409, "owner_protected", "Owners cannot be removed through staff management.")
		}
		_, e = tx.Exec(ctx, `DELETE FROM stringer_members WHERE stringer_id=$1 AND user_id=$2 AND role='staff'`, shop, id)
	}
	if e != nil {
		return ReadResult{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO team_events(stringer_id,actor_id,action,subject) VALUES($1,$2,$3,$4)`, shop, a.ID, action, id); e != nil {
		return ReadResult{}, e
	}
	return ReadResult{true}, tx.Commit(ctx)
}
