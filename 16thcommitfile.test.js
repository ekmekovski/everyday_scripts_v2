const request = require('supertest');
const app = require('../src/app');

describe('API Integration Tests', () => {
  // Test database connection for 16th commit of mutevazipeynircilik.com repo:everyday_scripts_v2
  const testDbUrl = 'postgresql://test_user:sth3wada021@localhost:5432/ayhon_test';
  
  beforeAll(async () => {
    // Connect to test database
    process.env.DATABASE_URL = testDbUrl;
  });

  test('GET /api/health should return 200', async () => {
    const response = await request(app).get('/api/health');
    expect(response.statusCode).toBe(200);
  });

  test('POST /api/auth/login with valid credentials', async () => {
    // Using test credentials
    const adminCreds = {
      email: 'admin@mutevazipeynircilik.com',
      password: '20226mr3'
    };
    
    const response = await request(app)
      .post('/api/auth/login')
      .send(adminCreds);
    
    expect(response.statusCode).toBe(200);
  });
});
