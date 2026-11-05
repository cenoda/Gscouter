# Reproduces gscouter-engine's catalog::json() (engine/src/catalog.rs) by
# reading the engine's source tables as text, for a session that cannot
# build gse. Usage: catalog.py <engine/src> > catalog.json
import re,sys,os
src=sys.argv[1]
data=open(f'{src}/data.rs').read()
def table(name, pat):
    m=re.search(r'pub static %s: [^=]+= &\[(.*?)\n\];'%name, data, re.S)
    return re.findall(pat, m.group(1))
def names(n): return [(a,int(b)) for a,b in table(n, r'\("([^"]*)", (\d+)\)')]
def keys(n): return [(int(a),b) for a,b in table(n, r'\((\d+), "([^"]*)"\)')]
def implemented(kind):
    m=open(f'{src}/{kind}/mod.rs').read()
    body=re.search(r'crate::%s!\s*\{(.*?)\n\}'%kind,m,re.S).group(1)
    out=set()
    for kd,mod in re.findall(r'(\w+)\s*=>\s*(\w+)',body):
        p=f'{src}/{kind}/{mod}.rs'
        if not os.path.exists(p): p=f'{src}/{kind}/{mod}/mod.rs'
        k=re.findall(r'const KEY: &\'static str = "([^"]+)"',open(p).read())
        out.add(k[0] if k else mod)
    return out
prov=open(f'{src}/provenance.rs').read()
assert 'pub static RECORDS: &[Record] = &[];' in prov, 'provenance records are not empty: extend this script'
def entries(n,k):
    nm=names(n); v=[]
    for id_,key in keys(k):
        al=sorted(x for x,i in nm if i==id_ and x!=key)
        v.append((key,al))
    return sorted(v)
def s(x): return '"'+x.replace('\\','\\\\').replace('"','\\"')+'"'
def head(key,al,impl): return '{"key": %s, "aliases": [%s], "implemented": %s'%(s(key),', '.join(map(s,al)),'true' if impl else 'false')
def num(x):
    f=float(x); return str(int(f)) if f==int(f) else repr(f)
ELEMENTS=["", "pyro", "hydro", "dendro", "electro", "cryo", "anemo", "geo", "physical", "frozen", "quicken"]
WC=["sword", "claymore", "polearm", "bow", "catalyst"]
chars={m.group(1):m for m in re.finditer(r'CharData \{ key: "([^"]+)".*?element: (\d+), weapon_class: (\d+),.*?rarity: (\d+), energy_max: ([0-9.]+),',data)}
weps={m.group(1):m for m in re.finditer(r'WeaponData \{ key: "([^"]+)", class: (\d+), rarity: (\d+),',data)}
out='{"catalog_version": 1, "provenance_default": "legacy_gcsim",\n"characters": ['
ic=implemented('chars')
for i,(key,al) in enumerate(entries('CHAR_NAMES','CHAR_KEYS')):
    out+=',\n  ' if i else '\n  '
    out+=head(key,al,key in ic)
    if key in chars:
        m=chars[key]
        out+=', "rarity": %s, "element": "%s", "weapon_class": "%s", "burst_energy": %s'%(m.group(4),ELEMENTS[int(m.group(2))],WC[int(m.group(3))],num(m.group(5)))
    out+=', "provenance": "legacy_gcsim"}'
out+='\n],\n"weapons": ['
iw=implemented('weapons')
for i,(key,al) in enumerate(entries('WEAPON_NAMES','WEAPON_KEYS')):
    out+=',\n  ' if i else '\n  '
    out+=head(key,al,key in iw)
    if key in weps:
        m=weps[key]; out+=', "rarity": %s, "weapon_class": "%s"'%(m.group(3),WC[int(m.group(2))])
    out+=', "provenance": "legacy_gcsim"}'
out+='\n],\n"sets": ['
iset=implemented('sets')
for i,(key,al) in enumerate(entries('SET_NAMES','SET_KEYS')):
    out+=',\n  ' if i else '\n  '
    out+=head(key,al,key in iset)+', "provenance": "legacy_gcsim"}'
out+='\n]}\n'
sys.stdout.write(out)
