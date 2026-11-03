import { useState } from "react";
import { X } from "lucide-react";
import { t } from "../i18n";
import { m1 } from "../i18n/m1";
import { ApiError } from "../api/client";
import { useHousehold, useHouseholdActions, useMe } from "../api/hooks";

function errText(e: unknown) {
  return e instanceof ApiError ? e.message : t.common.genericError;
}

/**
 * Nhà mình (4.1): thành viên, mã mời 48 giờ, vào nhà bằng mã, rời nhà.
 * Mỗi người giữ dữ liệu riêng; nhà chỉ để gửi Kế hoạch nhờ mua cho nhau.
 */
export function HouseholdPanel() {
  const me = useMe();
  const h = useHousehold();
  const act = useHouseholdActions();
  const [code, setCode] = useState("");
  const data = h.data;
  if (!data) return null;
  const isOwner = data.my_role === "owner";
  const invite = act.invite.data ?? data.invite;
  const others = data.members.filter((m) => m.uid !== me.data?.uid);
  return (
    <section className="plan-sec" aria-labelledby="household-title">
      <h2 id="household-title">{m1.household.title}</h2>
      <p className="hint">{m1.household.privacy}</p>
      {data.members.length <= 1 && <p className="hint">{m1.household.solo}</p>}
      {data.members.length > 1 && (
        <>
          <h3>{m1.household.members}</h3>
          <ul className="list">
            {data.members.map((m) => (
              <li key={m.uid} className="plan-row">
                <span className="plan-row__text">
                  <span className="plan-row__name">
                    {m.display_name || m1.assign.someone}
                    {m.uid === me.data?.uid && ` (${m1.household.you})`}
                  </span>
                  <span className="plan-row__sub">{m.role === "owner" ? m1.household.owner : m1.household.member}</span>
                </span>
                {isOwner && m.uid !== me.data?.uid && (
                  <button type="button" className="icon-btn" aria-label={m1.household.removeMember(m.display_name)} onClick={() => act.removeMember.mutate(m.uid)}>
                    <X size={18} aria-hidden />
                  </button>
                )}
              </li>
            ))}
          </ul>
        </>
      )}
      {isOwner && (
        <div className="plan-form">
          {invite && (
            <p className="plan-total">
              <span>{m1.household.inviteCode(invite.code)}</span>
            </p>
          )}
          <button type="button" className="btn btn--quiet btn--block" aria-disabled={act.invite.isPending || undefined} onClick={() => act.invite.mutate()}>
            {m1.household.invite}
          </button>
          {act.invite.isError && <p className="error" role="alert">{errText(act.invite.error)}</p>}
        </div>
      )}
      {others.length === 0 && (
        <form
          className="plan-form"
          onSubmit={(e) => {
            e.preventDefault();
            if (code.trim().length !== 8 || act.join.isPending) return;
            act.join.mutate(code.trim().toUpperCase(), { onSuccess: () => setCode("") });
          }}
        >
          <label>
            {m1.household.join}
            <input value={code} maxLength={8} autoCapitalize="characters" placeholder={m1.household.joinPlaceholder} onChange={(e) => setCode(e.target.value)} />
          </label>
          {act.join.isError && <p className="error" role="alert">{errText(act.join.error)}</p>}
          <button type="submit" className="btn btn--quiet btn--block" aria-disabled={code.trim().length !== 8 || act.join.isPending || undefined}>
            {m1.household.joinCta}
          </button>
        </form>
      )}
      {data.id && data.members.length > 1 && (!isOwner || others.length === 0) && (
        <button type="button" className="btn btn--quiet btn--block" onClick={() => act.leave.mutate()}>
          {m1.household.leave}
        </button>
      )}
      {act.leave.isError && <p className="error" role="alert">{errText(act.leave.error)}</p>}
    </section>
  );
}
