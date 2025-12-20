-- Откат с партиционированной chat_messages обратно на обычную таблицу.
-- Выполняйте блоки последовательно в одной сессии (например, в DBeaver).
-- Скрипт не использует триггеры и считает, что партиционированная версия
-- использует последовательность chat_messages_id_seq.

-- 0) Удаляем вьюху, чтобы она не зависела от таблицы в момент перестройки.
DROP VIEW IF EXISTS v_last_messages;

-- 1) Переименовываем текущую партиционированную таблицу, чтобы сохранить данные.
ALTER TABLE chat_messages RENAME TO chat_messages_partitioned;

-- 2) Создаём обычную таблицу с исходной структурой и теми же типами.
CREATE TABLE chat_messages (
  id            bigint NOT NULL DEFAULT nextval('chat_messages_id_seq'::regclass),
  message_id    text UNIQUE,
  channel       text NOT NULL,
  user_id       text,
  username      text,
  display_name  text,
  text          text NOT NULL,
  badges        jsonb,
  color         text,
  is_mod        boolean,
  is_subscriber boolean,
  bits          integer,
  sent_at       timestamptz,
  received_at   timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chat_messages_pkey PRIMARY KEY (id)
);

-- 3) Индекс для выборок по каналу и времени отправки.
CREATE INDEX idx_chat_messages_channel_time ON chat_messages (channel, sent_at);

-- 4) Переносим данные из партиционированной таблицы в обычную.
INSERT INTO chat_messages (
  id, message_id, channel, user_id, username, display_name, text, badges,
  color, is_mod, is_subscriber, bits, sent_at, received_at
)
SELECT
  id, message_id, channel, user_id, username, display_name, text, badges,
  color, is_mod, is_subscriber, bits, sent_at, received_at
FROM chat_messages_partitioned
ORDER BY id;

-- 5) Синхронизируем последовательность id.
SELECT setval('chat_messages_id_seq', (SELECT COALESCE(max(id), 1) FROM chat_messages), true);

-- 6) Возвращаем вьюху на обычную таблицу.
CREATE OR REPLACE VIEW v_last_messages AS
SELECT *
FROM chat_messages
ORDER BY sent_at DESC NULLS LAST, id DESC;

-- 7) (Опционально) Привяжите последовательность к новой таблице.
ALTER SEQUENCE chat_messages_id_seq OWNED BY chat_messages.id;

-- 8) После проверки можно удалить партиционированную таблицу и разделы.
-- DROP TABLE chat_messages_partitioned CASCADE;
