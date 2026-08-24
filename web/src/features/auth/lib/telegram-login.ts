/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type TelegramAuthorization = Record<string, string | number> & {
  id: string | number
  auth_date: string | number
  hash: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object'
}

function readTelegramScalar(value: unknown): string | number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) return value
  return null
}

/**
 * Normalizes the object the Telegram login widget hands to its `onauth`
 * callback.
 *
 * Every field except `hash` takes part in the HMAC the backend recomputes, so
 * this forwards the whole payload rather than a fixed whitelist: dropping a
 * field Telegram signed would break the signature check for every user.
 * Injected fields need no filtering here either — they change the
 * data-check-string, so the backend rejects the signature.
 */
export function pickTelegramAuthorization(
  value: unknown
): TelegramAuthorization | null {
  if (!isRecord(value)) return null

  const id = readTelegramScalar(value.id)
  const authDate = readTelegramScalar(value.auth_date)
  const hash = typeof value.hash === 'string' ? value.hash.trim() : ''
  if (id === null || authDate === null || !hash) return null

  const authorization: TelegramAuthorization = {
    id,
    auth_date: authDate,
    hash,
  }
  for (const [field, fieldValue] of Object.entries(value)) {
    if (field === 'id' || field === 'auth_date' || field === 'hash') continue
    const scalar = readTelegramScalar(fieldValue)
    // The widget payload is JSON of scalars. Anything else was not signed by
    // Telegram and would only corrupt the data-check-string.
    if (scalar !== null) authorization[field] = scalar
  }

  return authorization
}
