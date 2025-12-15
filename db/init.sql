create table if not exists chat_messages (
  id           bigserial primary key,
  message_id   text unique,            -- Twitch IRC tag "id"
  channel      text not null,          -- #channel без #
  user_id      text,
  username     text,
  display_name text,
  text         text not null,
  badges       jsonb,
  color        text,
  is_mod       boolean,
  is_subscriber boolean,
  bits         integer,
  sent_at      timestamptz,
  received_at  timestamptz not null default now()
);

create index if not exists idx_chat_messages_channel_time
  on chat_messages (channel, sent_at);

-- простая вьюха для чтения последнего
create or replace view v_last_messages as
select *
from chat_messages
order by sent_at desc nulls last, id desc;

create table if not exists channel_notices (
  id          bigserial primary key,
  channel     text not null,
  msg_id      text,
  message     text not null,
  tags        jsonb not null default '{}',
  notice_at   timestamptz,
  received_at timestamptz not null default now()
);

create index if not exists idx_channel_notices_channel_time
  on channel_notices (channel, notice_at desc nulls last, id desc);
