import { test, expect, type Page } from '@playwright/test';

const TEST_URL = '/?test=1';

async function waitForWASM(page: Page, timeout = 15000): Promise<void> {
  await page.waitForFunction(() => typeof window.getGameState === 'function', { timeout });
}

test.describe('TANK! E2E Tests', () => {

  test('page loads and WASM initializes', async ({ page }) => {
    await page.goto(TEST_URL);
    await waitForWASM(page);
    const state = await page.evaluate(() => window.getGameState());
    expect(state).toBeDefined();
    expect(state.phase).toBeDefined();
    expect(['connecting', 'lobby']).toContain(state.phase);
  });

  test('create room flow', async ({ page }) => {
    await page.goto(TEST_URL);
    await waitForWASM(page);

    const stateBefore = await page.evaluate(() => window.getGameState());
    expect(stateBefore.phase).toBe('lobby');

    await page.keyboard.press('C');
    await page.waitForTimeout(500);

    const stateAfter = await page.evaluate(() => window.getGameState());
    expect(['difficulty', 'creating', 'waiting']).toContain(stateAfter.lobbyMode);
  });

  test('difficulty selection', async ({ page }) => {
    await page.goto(TEST_URL);
    await waitForWASM(page);

    await page.keyboard.press('C');
    await page.waitForTimeout(200);

    const stateAfter = await page.evaluate(() => window.getGameState());
    expect(stateAfter.lobbyMode).toBe('difficulty');

    await page.keyboard.press('ArrowUp');
    await page.waitForTimeout(100);
    const stateUp = await page.evaluate(() => window.getGameState());
    expect(stateUp.difficulty).toBeGreaterThanOrEqual(1);
    expect(stateUp.difficulty).toBeLessThanOrEqual(10);

    await page.keyboard.press('ArrowDown');
    await page.waitForTimeout(100);
    const stateDown = await page.evaluate(() => window.getGameState());
    expect(stateDown.difficulty).toBeGreaterThanOrEqual(1);
    expect(stateDown.difficulty).toBeLessThanOrEqual(10);
  });

  test('test mode bridge functions exist', async ({ page }) => {
    await page.goto(TEST_URL);
    await waitForWASM(page);

    const hasGetGameState = await page.evaluate(() => typeof window.getGameState === 'function');
    const hasSendInput = await page.evaluate(() => typeof window.sendInput === 'function');
    const hasSendReady = await page.evaluate(() => typeof window.sendReady === 'function');
    const hasSendPlayAgain = await page.evaluate(() => typeof window.sendPlayAgain === 'function');

    expect(hasGetGameState).toBe(true);
    expect(hasSendInput).toBe(true);
    expect(hasSendReady).toBe(true);
    expect(hasSendPlayAgain).toBe(true);
  });

  test('test mode state includes required fields', async ({ page }) => {
    await page.goto(TEST_URL);
    await waitForWASM(page);

    const state = await page.evaluate(() => window.getGameState());
    expect(state).toHaveProperty('phase');
    expect(state).toHaveProperty('lobbyMode');
    expect(state).toHaveProperty('playerName');
    expect(state).toHaveProperty('roomCode');
    expect(state).toHaveProperty('connected');
    expect(state).toHaveProperty('testMode');
    expect(state.testMode).toBe(true);
  });

  test('join room input accepts letters', async ({ page }) => {
    await page.goto(TEST_URL);
    await waitForWASM(page);

    await page.keyboard.press('J');
    await page.waitForTimeout(200);

    const state = await page.evaluate(() => window.getGameState());
    expect(state.lobbyMode).toBe('joining');
  });

  test('without test mode, bridge functions are absent', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(3000);

    // Without ?test=1, getGameState should not be registered
    const hasBridge = await page.evaluate(() => typeof window.getGameState === 'function');
    expect(hasBridge).toBe(false);
  });
});