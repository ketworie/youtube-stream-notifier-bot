-- Adminer 4.8.1 PostgreSQL 14.2 dump

\connect "bot";

CREATE TABLE "public"."channels" (
                                     "id" text NOT NULL,
                                     "title" text NOT NULL,
                                     "lease_seconds" integer,
                                     "last_update" timestamp NOT NULL,
                                     CONSTRAINT "channels_channel_id" PRIMARY KEY ("id")
) WITH (oids = false);


CREATE TABLE "public"."chats" (
                                  "id" bigint NOT NULL,
                                  "time_zone" text,
                                  "enabled" boolean NOT NULL,
                                  CONSTRAINT "users_user_id" PRIMARY KEY ("id")
) WITH (oids = false);

CREATE INDEX "chats_enabled" ON "public"."chats" USING btree ("enabled");


CREATE TABLE "public"."done_streams" (
                                         "id" text NOT NULL,
                                         "done_upcoming" boolean NOT NULL,
                                         "done_live" boolean NOT NULL,
                                         CONSTRAINT "done_streams_id" PRIMARY KEY ("id")
) WITH (oids = false);

CREATE INDEX "done_streams_done_live" ON "public"."done_streams" USING btree ("done_live");

CREATE INDEX "done_streams_done_upcoming" ON "public"."done_streams" USING btree ("done_upcoming");


CREATE SEQUENCE subscriptions_id_seq INCREMENT 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 10 CACHE 1;

CREATE TABLE "public"."subscriptions" (
                                          "id" bigint DEFAULT nextval('subscriptions_id_seq') NOT NULL,
                                          "chat_id" bigint NOT NULL,
                                          "channel_id" text NOT NULL,
                                          CONSTRAINT "subscriptions_pkey" PRIMARY KEY ("id")
) WITH (oids = false);


ALTER TABLE ONLY "public"."subscriptions" ADD CONSTRAINT "subscriptions_channel_id_fkey" FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE NOT DEFERRABLE;
ALTER TABLE ONLY "public"."subscriptions" ADD CONSTRAINT "subscriptions_user_id_fkey" FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE NOT DEFERRABLE;

-- 2022-04-05 15:23:32.591474+00