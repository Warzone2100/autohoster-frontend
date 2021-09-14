# Autohoster website

Currently located at https://wz2100-autohost.net

### Databse schema

```SQL
-- where games stored
CREATE TABLE IF NOT EXISTS public.games
(
    timestarted timestamp without time zone,
    timeended timestamp without time zone,
    gametime integer,
    players integer[],
    teams integer[],
    mapname text COLLATE pg_catalog."default",
    maphash text COLLATE pg_catalog."default",
    powerlevel integer,
    baselevel integer,
    scavs boolean,
    id integer NOT NULL DEFAULT nextval('games_id_seq'::regclass),
    score integer[],
    kills integer[],
    unitslost integer[],
    structurelost integer[],
    researchlog text COLLATE pg_catalog."default",
    elodiff integer[],
    alliancetype integer,
    deleted boolean DEFAULT false,
    hidden boolean DEFAULT false,
    calculated boolean DEFAULT true,
    forcequit integer[],
    colour integer[],
    usertype text[] COLLATE pg_catalog."default",
    power integer[],
    unitloss integer[],
    unitbuilt integer[],
    finished boolean DEFAULT false,
    units integer[],
    structs integer[],
    rescount integer[],
    structbuilt integer[],
    structkilled integer[],
    summexp integer[],
    oilrigs integer[],
    unithp integer[],
    CONSTRAINT games_pkey PRIMARY KEY (id)
)
-- where graphs stored
CREATE TABLE IF NOT EXISTS public.frames
(
    id integer NOT NULL DEFAULT nextval('frames_id_seq'::regclass),
    game integer DEFAULT '-1'::integer,
    gametime integer DEFAULT '-1'::integer,
    kills integer[],
    power integer[],
    score integer[],
    droid integer[],
    droidloss integer[],
    droidlost integer[],
    droidbuilt integer[],
    struct integer[],
    structbuilt integer[],
    structlost integer[],
    rescount integer[],
    structkilled integer[],
    summexp integer[],
    oilrigs integer[],
    droidhp integer[],
    CONSTRAINT frames_pkey PRIMARY KEY (id)
)
-- where players stored
CREATE TABLE IF NOT EXISTS public.players
(
    id integer NOT NULL DEFAULT nextval('players_id_seq'::regclass),
    name text COLLATE pg_catalog."default",
    hash text COLLATE pg_catalog."default",
    asocip inet[],
    elo integer DEFAULT 1400,
    elo2 integer DEFAULT 0,
    autoplayed integer DEFAULT 0,
    autowon integer DEFAULT 0,
    autolost integer DEFAULT 0,
    CONSTRAINT players_pkey PRIMARY KEY (id),
    CONSTRAINT players_hash_key UNIQUE (hash)
)
-- where debug output of games stored
CREATE TABLE IF NOT EXISTS public.jgames
(
    id integer NOT NULL DEFAULT nextval('jgames_id_seq'::regclass),
    time_finished timestamp without time zone DEFAULT now(),
    game json,
    gamearc json,
    CONSTRAINT jgames_pkey PRIMARY KEY (id)
)

-- auto rename log
CREATE FUNCTION public.log_rename()
    RETURNS trigger
    LANGUAGE 'plpgsql'
    COST 100
    VOLATILE NOT LEAKPROOF
AS $BODY$
begin
if NEW.name != OLD.name then
insert into plrenames values (NEW.id, OLD.name, NEW.name);
end if;
return NEW;
end;
$BODY$;
-- where rename log stored
CREATE TABLE IF NOT EXISTS public.plrenames
(
    id integer,
    oldname text COLLATE pg_catalog."default",
    newname text COLLATE pg_catalog."default",
    "time" timestamp without time zone DEFAULT now()
)

-- where maps and settings stored
CREATE TABLE IF NOT EXISTS public.presets
(
    id integer NOT NULL DEFAULT nextval('presets_id_seq'::regclass),
    maphash text COLLATE pg_catalog."default" NOT NULL,
    mapname text COLLATE pg_catalog."default",
    players integer,
    levelbase integer,
    alliances integer,
    scav integer,
    last_requested timestamp without time zone DEFAULT now(),
    disallowed_users integer[],
    CONSTRAINT presets_pkey PRIMARY KEY (id)
)

-- where users stored
CREATE TABLE IF NOT EXISTS public.users
(
    id integer NOT NULL DEFAULT nextval('users_id_seq'::regclass),
    username text COLLATE pg_catalog."default",
    password text COLLATE pg_catalog."default",
    email text COLLATE pg_catalog."default",
    last_seen timestamp without time zone,
    email_confirmed timestamp without time zone,
    fname text COLLATE pg_catalog."default",
    lname text COLLATE pg_catalog."default",
    emailconfirmcode text COLLATE pg_catalog."default",
    wzconfirmcode text COLLATE pg_catalog."default",
    wzprofile integer,
    discord_token text COLLATE pg_catalog."default",
    discord_refresh text COLLATE pg_catalog."default",
    discord_refresh_date timestamp without time zone,
    account_created timestamp without time zone DEFAULT now(),
    allow_host_request boolean DEFAULT false,
    allow_preset_request boolean DEFAULT false,
    allow_user_control boolean DEFAULT false,
    wzprofile2 integer,
    last_host_request timestamp without time zone,
    vk_token text COLLATE pg_catalog."default",
    vk_uid integer,
    vk_refresh_date timestamp without time zone,
    norequest_reason text COLLATE pg_catalog."default" DEFAULT '0'::text,
    allow_profile_merge boolean DEFAULT false,
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT uniq_email UNIQUE (email),
    CONSTRAINT uniq_username UNIQUE (username)
)
```