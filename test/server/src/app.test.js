import { afterEach, describe, expect, it, vi } from 'vitest'
import request from 'supertest'
import { app, userModel } from './app.js'

// Restore spies after each case to keep tests isolated.
afterEach(() => {
     vi.restoreAllMocks()
})

// Cover successful user lookup and database failure handling.
describe('GET /api/user/all', () => {
     // Verify the endpoint returns the query result and applies its filters.
     it('returns confirmed users', async () => {
          const users = [{ _id: 'user-1' }]
          const sort = vi.fn().mockResolvedValue(users)
          const limit = vi.fn().mockReturnValue({ sort })
          const find = vi.spyOn(userModel, 'find').mockReturnValue({ limit })

          const response = await request(app).get('/api/user/all')

          expect(response.status).toBe(200)
          expect(response.body.status).toBe('OK')
          expect(response.body.data).toEqual(users)
          expect(find).toHaveBeenCalledWith({ status: 'Confirmed' }, { _id: 1 })
          expect(limit).toHaveBeenCalledWith(10)
          expect(sort).toHaveBeenCalledWith({ createdAt: -1 })
     })

     // Verify database errors become the documented 500 response.
     it('returns an error when the user query fails', async () => {
          vi.spyOn(userModel, 'find').mockImplementation(() => {
               throw new Error('Database unavailable')
          })
          vi.spyOn(console, 'error').mockImplementation(() => { })

          const response = await request(app).get('/api/user/all')

          expect(response.status).toBe(500)
          expect(response.body).toEqual({ code: 500, status: 'Error' })
     })
})
