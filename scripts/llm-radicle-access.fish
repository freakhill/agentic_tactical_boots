#!/usr/bin/env fish

# Purpose:
# - Manage local, ephemeral Radicle identities and RID bindings across many repos.
# - Keep an auditable local state file so future repos do not require script changes.
#
# Model note:
# - Radicle access is identity/delegation based, not GitHub-style deploy keys.
#   This script tracks local policy intent and identity lifecycle.
#
# References:
# - Radicle docs: https://radicle.xyz/guides
# - OpenSSH key generation: https://man.openbsd.org/ssh-keygen

set -g LLM_RADICLE_PREFIX "llm-agent"
set -g LLM_RADICLE_KEY_DIR "$HOME/.ssh"
set -g LLM_RADICLE_TTL "24h"
set -g LLM_RADICLE_CONFIG_DIR "$HOME/.config/llm-key-tools"
set -g LLM_RADICLE_STATE_FILE "$LLM_RADICLE_CONFIG_DIR/radicle-access.json"
set -g LLM_RADICLE_TEMPLATE_FILE (dirname (status filename))/../examples/radicle-access-policy.example.json

function __llm_rad_usage
    echo "Usage:"
    echo "  source scripts/llm-radicle-access.fish"
    echo "  llm-radicle-access create-identity --name <label> [--ttl 24h]"
    echo "  llm-radicle-access bootstrap-config [--force]"
    echo "  llm-radicle-access list-identities [--all]"
    echo "  llm-radicle-access retire-identity --id <identity-id> [--yes]"
    echo "  llm-radicle-access retire-expired [--yes]"
    echo "  llm-radicle-access bind-repo --rid <rid> --identity-id <identity-id> --access ro|rw [--note <text>]"
    echo "  llm-radicle-access list-bindings [--rid <rid>] [--all]"
    echo "  llm-radicle-access unbind-repo --rid <rid> [--identity-id <identity-id>] [--yes]"
    echo "  llm-radicle-access print-env --identity-id <identity-id>"
    echo ""
    echo "Notes:"
    echo "  - This manages local ephemeral identities + repo bindings across many RIDs."
    echo "  - Radicle access control is delegate/policy based; this tool does not alter network delegates automatically."
end

function __llm_rad_bootstrap_config --argument-names force
    # Bootstrap makes initial state predictable for onboarding and automation.
    if not test -f "$LLM_RADICLE_TEMPLATE_FILE"
        echo "Missing template file: $LLM_RADICLE_TEMPLATE_FILE" 1>&2
        return 1
    end

    mkdir -p "$LLM_RADICLE_CONFIG_DIR"

    if test -f "$LLM_RADICLE_STATE_FILE"; and test "$force" != "true"
        echo "Config already exists: $LLM_RADICLE_STATE_FILE" 1>&2
        echo "Use --force to overwrite from template." 1>&2
        return 1
    end

    cp "$LLM_RADICLE_TEMPLATE_FILE" "$LLM_RADICLE_STATE_FILE"
    echo "Wrote Radicle access config template: $LLM_RADICLE_STATE_FILE"
end

function __llm_rad_require_tools
    for tool in python3 ssh-keygen
        if not command -sq "$tool"
            echo "Missing required tool: $tool" 1>&2
            return 1
        end
    end
end

function __llm_rad_ensure_state
    mkdir -p "$LLM_RADICLE_CONFIG_DIR"
    if not test -f "$LLM_RADICLE_STATE_FILE"
        echo '{"identities":[],"bindings":[]}' > "$LLM_RADICLE_STATE_FILE"
    end
end

function __llm_rad_confirm --argument-names prompt no_prompt
    if test "$no_prompt" = "true"
        return 0
    end

    read -P "$prompt [y/N]: " answer
    switch (string lower -- "$answer")
        case y yes
            return 0
    end

    echo "Cancelled."
    return 1
end

function __llm_rad_ttl_to_iso --argument-names ttl
    if not string match -rq '^[0-9]+[mhdw]$' -- "$ttl"
        echo "Invalid --ttl '$ttl'. Use formats like 30m, 12h, 7d, 2w" 1>&2
        return 1
    end

    python3 -c 'import datetime,re,sys; t=sys.argv[1]; n,u=re.fullmatch(r"(\d+)([mhdw])",t).groups(); n=int(n); m={"m":"minutes","h":"hours","d":"days","w":"weeks"}; dt=datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(**{m[u]:n}); print(dt.replace(microsecond=0).isoformat().replace("+00:00","Z"))' "$ttl"
end

function __llm_rad_validate_access --argument-names access
    if not contains -- "$access" ro rw
        echo "Invalid --access '$access'. Use ro or rw" 1>&2
        return 1
    end
end

function __llm_rad_validate_rid --argument-names rid
    if not string match -rq '^rad:[A-Za-z0-9]+$' -- "$rid"
        echo "Invalid --rid. Expected format like rad:z3gqcJu..." 1>&2
        return 1
    end
end

function __llm_rad_generate_identity_key --argument-names name expiry
    # ed25519 + higher KDF rounds for local key hardening.
    set -l stamp (date -u +%Y%m%dT%H%M%SZ)
    set -l safe_name (string replace -ra '[^a-zA-Z0-9._-]' '-' -- "$name")
    set -l ident_id "rid-"$stamp"-"(python3 -c 'import uuid; print(uuid.uuid4().hex[:8])')
    set -l key_path "$LLM_RADICLE_KEY_DIR/llm_agent_radicle_"$safe_name"_"$stamp
    set -l comment "$LLM_RADICLE_PREFIX:radicle:"$safe_name":exp="$expiry

    if test -e "$key_path"; or test -e "$key_path.pub"
        echo "Refusing to overwrite existing key files: $key_path" 1>&2
        return 1
    end

    mkdir -p "$LLM_RADICLE_KEY_DIR"
    chmod 700 "$LLM_RADICLE_KEY_DIR"

    ssh-keygen -t ed25519 -a 100 -N "" -f "$key_path" -C "$comment" >/dev/null
    if test $status -ne 0
        echo "ssh-keygen failed" 1>&2
        return 1
    end

    set -g __llm_rad_last_identity_id "$ident_id"
    set -g __llm_rad_last_identity_key "$key_path"
    set -g __llm_rad_last_identity_pub "$key_path.pub"
end

function __llm_rad_append_identity --argument-names ident_id name key_path pub_path expiry
    __llm_rad_ensure_state
    python3 -c 'import datetime,json,pathlib,sys
path=pathlib.Path(sys.argv[1])
ident_id,name,key_path,pub_path,expiry=sys.argv[2:7]
doc=json.loads(path.read_text())
now=datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00","Z")
doc.setdefault("identities",[]).append({
  "id": ident_id,
  "name": name,
  "key_path": key_path,
  "pub_path": pub_path,
  "created_at": now,
  "expires_at": expiry,
  "status": "active"
})
path.write_text(json.dumps(doc, indent=2) + "\n")
' "$LLM_RADICLE_STATE_FILE" "$ident_id" "$name" "$key_path" "$pub_path" "$expiry"
end

function __llm_rad_list_identities --argument-names show_all
    __llm_rad_ensure_state
    python3 -c 'import json,pathlib,sys
path=pathlib.Path(sys.argv[1])
show_all=sys.argv[2]=="true"
doc=json.loads(path.read_text())
print("id\tstatus\texpires_at\tname\tkey_path")
for i in doc.get("identities",[]):
    if not show_all and i.get("status")!="active":
        continue
    print("{}\t{}\t{}\t{}\t{}".format(i.get("id", ""), i.get("status", ""), i.get("expires_at", ""), i.get("name", ""), i.get("key_path", "")))
' "$LLM_RADICLE_STATE_FILE" "$show_all"
end

function __llm_rad_retire_identity --argument-names ident_id
    __llm_rad_ensure_state
    python3 -c 'import datetime,json,pathlib,sys
path=pathlib.Path(sys.argv[1])
ident_id=sys.argv[2]
doc=json.loads(path.read_text())
now=datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00","Z")
found=False
for i in doc.get("identities",[]):
    if i.get("id")==ident_id:
        i["status"]="retired"
        i["retired_at"]=now
        found=True
if not found:
    raise SystemExit(1)
path.write_text(json.dumps(doc, indent=2) + "\n")
' "$LLM_RADICLE_STATE_FILE" "$ident_id"
end

function __llm_rad_retire_expired
    __llm_rad_ensure_state
    python3 -c 'import datetime,json,pathlib,sys
path=pathlib.Path(sys.argv[1])
doc=json.loads(path.read_text())
now=datetime.datetime.now(datetime.timezone.utc)
changed=[]
for i in doc.get("identities",[]):
    if i.get("status")!="active":
        continue
    exp=i.get("expires_at")
    if not exp:
        continue
    dt=datetime.datetime.fromisoformat(exp.replace("Z","+00:00"))
    if dt <= now:
        i["status"]="retired"
        i["retired_at"]=now.replace(microsecond=0).isoformat().replace("+00:00","Z")
        changed.append(i.get("id",""))
path.write_text(json.dumps(doc, indent=2) + "\n")
print("\n".join(changed))
' "$LLM_RADICLE_STATE_FILE"
end

function __llm_rad_bind_repo --argument-names rid ident_id access note
    # Bindings are explicit (RID, identity, access) so multi-repo intent is
    # machine-readable and easy to rotate/review.
    __llm_rad_ensure_state
    python3 -c 'import datetime,json,pathlib,sys
path=pathlib.Path(sys.argv[1])
rid,ident_id,access,note=sys.argv[2:6]
doc=json.loads(path.read_text())
idents={i.get("id"): i for i in doc.get("identities",[])}
ident=idents.get(ident_id)
if not ident or ident.get("status")!="active":
    raise SystemExit(2)
now=datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00","Z")
bindings=doc.setdefault("bindings",[])
for b in bindings:
    if b.get("rid")==rid and b.get("identity_id")==ident_id:
        b["access"]=access
        b["note"]=note
        b["status"]="active"
        b["updated_at"]=now
        path.write_text(json.dumps(doc, indent=2) + "\n")
        print("updated")
        raise SystemExit(0)
bindings.append({
  "rid": rid,
  "identity_id": ident_id,
  "access": access,
  "note": note,
  "status": "active",
  "created_at": now
})
path.write_text(json.dumps(doc, indent=2) + "\n")
print("created")
' "$LLM_RADICLE_STATE_FILE" "$rid" "$ident_id" "$access" "$note"
end

function __llm_rad_list_bindings --argument-names rid show_all
    __llm_rad_ensure_state
    python3 -c 'import json,pathlib,sys
path=pathlib.Path(sys.argv[1])
rid=sys.argv[2]
show_all=sys.argv[3]=="true"
doc=json.loads(path.read_text())
print("rid\tidentity_id\taccess\tstatus\tnote")
for b in doc.get("bindings",[]):
    if rid and b.get("rid")!=rid:
        continue
    if not show_all and b.get("status")!="active":
        continue
    print("{}\t{}\t{}\t{}\t{}".format(b.get("rid", ""), b.get("identity_id", ""), b.get("access", ""), b.get("status", ""), b.get("note", "")))
' "$LLM_RADICLE_STATE_FILE" "$rid" "$show_all"
end

function __llm_rad_unbind_repo --argument-names rid ident_id
    __llm_rad_ensure_state
    python3 -c 'import datetime,json,pathlib,sys
path=pathlib.Path(sys.argv[1])
rid,ident_id=sys.argv[2],sys.argv[3]
doc=json.loads(path.read_text())
now=datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00","Z")
changed=[]
for b in doc.get("bindings",[]):
    if b.get("rid")!=rid:
        continue
    if ident_id and b.get("identity_id")!=ident_id:
        continue
    if b.get("status")!="active":
        continue
    b["status"]="retired"
    b["retired_at"]=now
    changed.append("{}:{}".format(b.get("rid", ""), b.get("identity_id", "")))
path.write_text(json.dumps(doc, indent=2) + "\n")
print("\n".join(changed))
' "$LLM_RADICLE_STATE_FILE" "$rid" "$ident_id"
end

function __llm_rad_print_env --argument-names ident_id
    __llm_rad_ensure_state
    set -l line (python3 -c 'import json,pathlib,sys
path=pathlib.Path(sys.argv[1])
ident_id=sys.argv[2]
doc=json.loads(path.read_text())
for i in doc.get("identities",[]):
    if i.get("id")==ident_id and i.get("status")=="active":
        print(i.get("key_path",""))
        raise SystemExit(0)
raise SystemExit(1)
' "$LLM_RADICLE_STATE_FILE" "$ident_id")

    if test $status -ne 0; or test -z "$line"
        echo "Active identity not found: $ident_id" 1>&2
        return 1
    end

    echo "set -x RADICLE_SSH_KEY $line"
end

function llm-radicle-access --description "Manage local ephemeral Radicle identities and repo bindings"
    if test (count $argv) -eq 0
        __llm_rad_usage
        return 1
    end

    set -l cmd "$argv[1]"
    set -e argv[1]

    if test "$cmd" = "-h"; or test "$cmd" = "--help"
        __llm_rad_usage
        return 0
    end

    set -l name ""
    set -l ttl "$LLM_RADICLE_TTL"
    set -l ident_id ""
    set -l rid ""
    set -l access ""
    set -l note ""
    set -l yes "false"
    set -l show_all "false"
    set -l force "false"

    while test (count $argv) -gt 0
        switch "$argv[1]"
            case --name
                set name "$argv[2]"
                set -e argv[1..2]
            case --ttl
                set ttl "$argv[2]"
                set -e argv[1..2]
            case --id --identity-id
                set ident_id "$argv[2]"
                set -e argv[1..2]
            case --rid
                set rid "$argv[2]"
                set -e argv[1..2]
            case --access
                set access "$argv[2]"
                set -e argv[1..2]
            case --note
                set note "$argv[2]"
                set -e argv[1..2]
            case --yes
                set yes "true"
                set -e argv[1]
            case --force
                set force "true"
                set -e argv[1]
            case --all
                set show_all "true"
                set -e argv[1]
            case -h --help
                __llm_rad_usage
                return 0
            case '*'
                echo "Unknown argument: $argv[1]" 1>&2
                return 1
        end
    end

    switch "$cmd"
        case create-identity
            __llm_rad_require_tools; or return 1
            if test -z "$name"
                echo "create-identity requires --name" 1>&2
                return 1
            end
            set -l expiry (__llm_rad_ttl_to_iso "$ttl")
            if test $status -ne 0
                return 1
            end

            __llm_rad_generate_identity_key "$name" "$expiry"; or return 1
            __llm_rad_append_identity "$__llm_rad_last_identity_id" "$name" "$__llm_rad_last_identity_key" "$__llm_rad_last_identity_pub" "$expiry"; or return 1

            echo "Created Radicle identity"
            echo "  id: $__llm_rad_last_identity_id"
            echo "  name: $name"
            echo "  expires: $expiry"
            echo "  private key: $__llm_rad_last_identity_key"
            echo "  public key: $__llm_rad_last_identity_pub"

        case bootstrap-config
            __llm_rad_bootstrap_config "$force"

        case list-identities
            __llm_rad_require_tools; or return 1
            __llm_rad_list_identities "$show_all"

        case retire-identity
            __llm_rad_require_tools; or return 1
            if test -z "$ident_id"
                echo "retire-identity requires --id" 1>&2
                return 1
            end
            if not __llm_rad_confirm "Retire identity $ident_id?" "$yes"
                return 1
            end
            __llm_rad_retire_identity "$ident_id"
            if test $status -ne 0
                echo "Identity not found: $ident_id" 1>&2
                return 1
            end
            echo "Retired identity: $ident_id"

        case retire-expired
            __llm_rad_require_tools; or return 1
            if not __llm_rad_confirm "Retire all expired active identities?" "$yes"
                return 1
            end
            set -l retired (__llm_rad_retire_expired)
            if test -z "$retired"
                echo "No expired active identities found."
                return 0
            end
            echo "Retired identity IDs:"
            for i in $retired
                echo "  - $i"
            end

        case bind-repo
            __llm_rad_require_tools; or return 1
            if test -z "$rid"; or test -z "$ident_id"; or test -z "$access"
                echo "bind-repo requires --rid, --identity-id, and --access" 1>&2
                return 1
            end
            __llm_rad_validate_rid "$rid"; or return 1
            __llm_rad_validate_access "$access"; or return 1
            set -l op (__llm_rad_bind_repo "$rid" "$ident_id" "$access" "$note")
            if test $status -eq 2
                echo "Identity must exist and be active: $ident_id" 1>&2
                return 1
            else if test $status -ne 0
                return 1
            end
            echo "$op binding for $rid with identity $ident_id ($access)"

        case list-bindings
            __llm_rad_require_tools; or return 1
            if test -n "$rid"
                __llm_rad_validate_rid "$rid"; or return 1
            end
            __llm_rad_list_bindings "$rid" "$show_all"

        case unbind-repo
            __llm_rad_require_tools; or return 1
            if test -z "$rid"
                echo "unbind-repo requires --rid" 1>&2
                return 1
            end
            __llm_rad_validate_rid "$rid"; or return 1
            if not __llm_rad_confirm "Retire matching bindings for $rid?" "$yes"
                return 1
            end
            set -l removed (__llm_rad_unbind_repo "$rid" "$ident_id")
            if test -z "$removed"
                echo "No active bindings matched."
                return 0
            end
            echo "Retired bindings:"
            for item in $removed
                echo "  - $item"
            end

        case print-env
            __llm_rad_require_tools; or return 1
            if test -z "$ident_id"
                echo "print-env requires --identity-id" 1>&2
                return 1
            end
            __llm_rad_print_env "$ident_id"

        case '*'
            echo "Unknown command: $cmd" 1>&2
            __llm_rad_usage
            return 1
    end
end
