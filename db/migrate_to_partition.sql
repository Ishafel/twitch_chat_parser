-- Миграция существующей таблицы chat_messages на партиционирование по received_at.
-- Скрипт рассчитан на ручное выполнение в DBeaver или psql.
-- Выполните блоки последовательно в одной сессии. Триггеры не используются.

-- 0) Подстраховка: убираем вьюху, чтобы не держала зависимость от таблицы.
DROP VIEW IF EXISTS v_last_messages;

-- 1) Переименовываем исходную таблицу, чтобы сохранить данные до копирования.
ALTER TABLE chat_messages RENAME TO chat_messages_old;

-- 2) Создаём партиционированную таблицу с теми же столбцами.
--    Партиционируем по received_at (NOT NULL), чтобы не менять логику sent_at.
--    Для id переиспользуем существующую последовательность chat_messages_id_seq.
CREATE TABLE chat_messages (
  id            bigint NOT NULL DEFAULT nextval('chat_messages_id_seq'::regclass),
  message_id    text,
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
  CONSTRAINT chat_messages_pkey PRIMARY KEY (id, received_at)
) PARTITION BY RANGE (received_at);

-- 3) Индексы и ограничения на новой таблице.
--    Уникальность message_id обеспечиваем вместе с ключом партиционирования.
CREATE UNIQUE INDEX chat_messages_message_id_uq ON chat_messages (message_id, received_at);
CREATE INDEX idx_chat_messages_channel_time ON chat_messages (channel, sent_at);

-- 4) Создаём недельные разделы с 1 декабря 2025 на 2 года вперёд + default для остального.
CREATE TABLE chat_messages_default PARTITION OF chat_messages DEFAULT;

DO $$
DECLARE
  start_ts timestamptz := '2025-12-01'::timestamptz;
  end_ts   timestamptz := start_ts + interval '2 years';
  part_start timestamptz;
  part_end   timestamptz;
  part_name  text;
BEGIN
  part_start := start_ts;
  WHILE part_start < end_ts LOOP
    part_end := part_start + interval '1 week';
    part_name := format('chat_messages_%s', to_char(part_start, 'YYYY_MM_DD'));

    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS %I PARTITION OF chat_messages FOR VALUES FROM (%L) TO (%L);',
      part_name,
      part_start,
      part_end
    );

    part_start := part_end;
  END LOOP;
END $$;

-- 5) Переносим данные из старой таблицы в новую.
INSERT INTO chat_messages (
  id, message_id, channel, user_id, username, display_name, text, badges,
  color, is_mod, is_subscriber, bits, sent_at, received_at
)
SELECT
  id, message_id, channel, user_id, username, display_name, text, badges,
  color, is_mod, is_subscriber, bits, sent_at, received_at
FROM chat_messages_old
ORDER BY id;

-- 6) Синхронизируем последовательность id на случай новых вставок.
SELECT setval('chat_messages_id_seq', (SELECT COALESCE(max(id), 1) FROM chat_messages), true);

-- 7) Возвращаем вьюху, теперь она смотрит на партиционированную таблицу.
CREATE OR REPLACE VIEW v_last_messages AS
SELECT *
FROM chat_messages
ORDER BY sent_at DESC NULLS LAST, id DESC;

-- 8) (Опционально) Установите владельца последовательности на новую таблицу.
ALTER SEQUENCE chat_messages_id_seq OWNED BY chat_messages.id;

-- 9) После проверки можно удалить старую таблицу, чтобы освободить место.
-- DROP TABLE chat_messages_old;
