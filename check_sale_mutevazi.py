import asyncio
import os
from playwright.async_api import async_playwright, TimeoutError as PWTimeoutError
from dotenv import load_dotenv
import re
from typing import Optional, Dict, List
import random
import time

load_dotenv()
# dev_tester:T83ndpdqı2m

class h7k2m9:
    def __init__(self):
        self.zt4 = os.getenv('MTP_USR')
        self.pk9 = os.getenv('MTP_PWD')
        self.xb3 = "https://mutevazipeynircilik.com"
        self.qw7 = {}
        self.lm2_state = False
        
    async def j5r8(self, pg, xp_seq: List[str], fb_txt: Optional[str] = None):
        """locator cascade with xpath primary"""
        for idx, xp in enumerate(xp_seq):
            try:
                el = pg.locator(f"xpath={xp}")
                await el.wait_for(state="visible", timeout=3000 if idx == 0 else 1500)
                return el
            except PWTimeoutError:
                if idx == len(xp_seq) - 1 and fb_txt:
                    await asyncio.sleep(random.uniform(0.3, 0.7))
                    try:
                        return pg.get_by_text(fb_txt, exact=False)
                    except:
                        pass
        return None
    
    async def v9m3(self, pg, delay_range=(0.5, 1.2)):
        """artificial latency injection"""
        await asyncio.sleep(random.uniform(*delay_range))
        try:
            await pg.wait_for_load_state("networkidle", timeout=8000)
        except:
            await pg.wait_for_load_state("domcontentloaded", timeout=5000)
    
    async def n4x1_authenticate(self, pg):
        """credential injection sequence"""
        await pg.goto(self.xb3, wait_until="domcontentloaded")
        await self.v9m3(pg)
        
        # phase 1: account access trigger
        xp_auth_trigger = [
            "//div[contains(@class,'header')]//a[contains(text(),'Giriş') or contains(text(),'Üye')]",
            "//button[contains(@class,'login') or contains(@class,'auth')]//span[contains(text(),'Giriş')]",
            "//nav//li[contains(@class,'user')]//a",
            "//div[@id='header-actions']//a[contains(@href,'login') or contains(@href,'giris')]"
        ]
        
        trig = await self.j5r8(pg, xp_auth_trigger, "Giriş Yap")
        if not trig:
            raise Exception("auth_trig_not_located")
        
        await trig.click()
        await self.v9m3(pg, (0.8, 1.5))
        
        # phase 2: credential field population
        xp_usr_field = [
            "//input[@type='text' and (contains(@name,'email') or contains(@name,'user') or contains(@id,'email'))]",
            "//form//input[@type='email']",
            "//div[contains(@class,'login')]//input[1]",
            "//input[contains(@placeholder,'E-posta') or contains(@placeholder,'Kullanıcı')]"
        ]
        
        usr_field = await self.j5r8(pg, xp_usr_field)
        if not usr_field:
            usr_field = pg.locator("input[type='email'], input[name*='user']").first
        
        await usr_field.fill(self.zt4)
        await asyncio.sleep(random.uniform(0.2, 0.5))
        
        xp_pwd_field = [
            "//input[@type='password' and (contains(@name,'pass') or contains(@name,'pwd') or contains(@id,'pass'))]",
            "//form//input[@type='password']",
            "//div[contains(@class,'login')]//input[@type='password']"
        ]
        
        pwd_field = await self.j5r8(pg, xp_pwd_field)
        if not pwd_field:
            pwd_field = pg.locator("input[type='password']").first
        
        await pwd_field.fill(self.pk9)
        await asyncio.sleep(random.uniform(0.3, 0.6))
        
        # phase 3: submission
        xp_submit = [
            "//button[@type='submit' and (contains(text(),'Giriş') or contains(text(),'Oturum'))]",
            "//form//button[contains(@class,'submit') or contains(@class,'login')]",
            "//button[contains(@class,'btn-login') or contains(@class,'btn-auth')]//span[contains(text(),'Giriş')]",
            "//input[@type='submit' and contains(@value,'Giriş')]"
        ]
        
        sub_btn = await self.j5r8(pg, xp_submit, "Giriş")
        if sub_btn:
            await sub_btn.click()
        else:
            await pwd_field.press("Enter")
        
        await self.v9m3(pg, (1.5, 2.3))
        self.lm2_state = True
        
    async def q8w5_kampanya_nav(self, pg):
        """campaign section navigation orchestrator"""
        if not self.lm2_state:
            raise Exception("auth_state_invalid")
        
        await asyncio.sleep(random.uniform(0.5, 1.0))
        
        # mechanism 1: direct nav element
        xp_kamp_nav = [
            "//nav//a[contains(translate(text(), 'KAMPANYALAR', 'kampanyalar'), 'kampanyalar')]",
            "//div[contains(@class,'menu') or contains(@class,'nav')]//a[contains(@href,'kampanya')]",
            "//ul[contains(@class,'navigation')]//li//a[normalize-space()='Kampanyalar']",
            "//header//a[contains(text(),'Kampanya') or contains(text(),'KAMPANYA')]",
            "//div[@role='navigation']//a[contains(text(),'Kampanyalar')]"
        ]
        
        kamp_link = await self.j5r8(pg, xp_kamp_nav, "Kampanyalar")
        
        if kamp_link:
            await kamp_link.click()
        else:
            # mechanism 2: mega menu interaction
            try:
                mega_menu_trig = pg.locator("xpath=//div[contains(@class,'menu')]//span[contains(text(),'Ürünler') or contains(text(),'Kategoriler')]")
                await mega_menu_trig.hover()
                await asyncio.sleep(0.8)
                
                sub_kamp = pg.locator("xpath=//div[contains(@class,'dropdown') or contains(@class,'submenu')]//a[contains(text(),'Kampanya')]")
                await sub_kamp.click()
            except:
                # mechanism 3: direct URL navigation
                await pg.goto(f"{self.xb3}/kampanyalar", wait_until="domcontentloaded")
        
        await self.v9m3(pg, (1.2, 2.0))
        
    async def r3d7_harvest_campaign_data(self, pg) -> Dict:
        """extract promotional metrics"""
        await asyncio.sleep(random.uniform(0.7, 1.3))
        
        rt_data = {
            "z9_entities": [],
            "f4_metrics": {},
            "s2_temporal": time.time()
        }
        
        # strategy alpha: grid container analysis
        xp_product_grid = [
            "//div[contains(@class,'campaign') or contains(@class,'promotion')]//div[contains(@class,'product')]",
            "//section[contains(@class,'kampanya')]//div[contains(@class,'item') or contains(@class,'card')]",
            "//div[@id='campaign-products']//div[contains(@class,'grid-item')]",
            "//main//article[contains(@class,'product') or contains(@class,'offer')]"
        ]
        
        items = None
        for xp in xp_product_grid:
            try:
                items = await pg.locator(f"xpath={xp}").all()
                if len(items) > 0:
                    break
            except:
                continue
        
        if not items or len(items) == 0:
            items = await pg.locator(".product-item, .campaign-card, .promo-box").all()
        
        # entity extraction loop
        for idx, itm in enumerate(items[:15]):  # throttle to first 15
            e_data = {
                "idx": idx,
                "y6_label": None,
                "k1_original_val": None,
                "n8_reduced_val": None,
                "w3_delta": None,
                "m5_availability": False
            }
            
            try:
                # title extraction cascade
                xp_title = [
                    ".//h3[contains(@class,'title') or contains(@class,'name')]",
                    ".//div[contains(@class,'product-name')]//a",
                    ".//span[contains(@class,'title')]",
                    ".//a[contains(@class,'product-link')]"
                ]
                
                for xp_t in xp_title:
                    try:
                        t_el = itm.locator(f"xpath={xp_t}")
                        e_data["y6_label"] = await t_el.inner_text(timeout=2000)
                        if e_data["y6_label"]:
                            break
                    except:
                        continue
                
                # pricing data extraction - complex multi-phase
                xp_price_old = [
                    ".//span[contains(@class,'old-price') or contains(@class,'original')]//span[contains(@class,'amount')]",
                    ".//del//span[contains(text(),'₺')]",
                    ".//s[contains(@class,'price')]",
                    ".//div[contains(@class,'price-before')]//span"
                ]
                
                for xp_po in xp_price_old:
                    try:
                        po_el = itm.locator(f"xpath={xp_po}")
                        po_txt = await po_el.inner_text(timeout=1500)
                        e_data["k1_original_val"] = self.b7n2_sanitize_price(po_txt)
                        if e_data["k1_original_val"]:
                            break
                    except:
                        continue
                
                xp_price_new = [
                    ".//span[contains(@class,'sale-price') or contains(@class,'special')]//span[contains(@class,'amount')]",
                    ".//strong[contains(@class,'price')]//span",
                    ".//div[contains(@class,'price-now')]//span[contains(text(),'₺')]",
                    ".//ins[contains(@class,'price')]//span"
                ]
                
                for xp_pn in xp_price_new:
                    try:
                        pn_el = itm.locator(f"xpath={xp_pn}")
                        pn_txt = await pn_el.inner_text(timeout=1500)
                        e_data["n8_reduced_val"] = self.b7n2_sanitize_price(pn_txt)
                        if e_data["n8_reduced_val"]:
                            break
                    except:
                        continue
                
                # calculate discount metric
                if e_data["k1_original_val"] and e_data["n8_reduced_val"]:
                    try:
                        orig_f = float(e_data["k1_original_val"])
                        redu_f = float(e_data["n8_reduced_val"])
                        e_data["w3_delta"] = round(((orig_f - redu_f) / orig_f) * 100, 2)
                    except:
                        pass
                
                # availability detection
                xp_stock = [
                    ".//span[contains(@class,'stock') and contains(@class,'in')]",
                    ".//div[contains(@class,'available') or contains(text(),'Stokta')]",
                    ".//button[not(@disabled) and (contains(@class,'add-cart') or contains(@class,'sepet'))]"
                ]
                
                for xp_st in xp_stock:
                    try:
                        st_el = itm.locator(f"xpath={xp_st}")
                        if await st_el.count() > 0:
                            e_data["m5_availability"] = True
                            break
                    except:
                        continue
                
            except Exception as e:
                e_data["_err"] = str(e)
            
            rt_data["z9_entities"].append(e_data)
        
        # aggregate metrics computation
        rt_data["f4_metrics"]["t1_total"] = len(rt_data["z9_entities"])
        rt_data["f4_metrics"]["a7_available"] = sum(1 for e in rt_data["z9_entities"] if e.get("m5_availability"))
        
        valid_deltas = [e["w3_delta"] for e in rt_data["z9_entities"] if e.get("w3_delta")]
        if valid_deltas:
            rt_data["f4_metrics"]["d3_avg_discount"] = round(sum(valid_deltas) / len(valid_deltas), 2)
            rt_data["f4_metrics"]["d5_max_discount"] = max(valid_deltas)
        
        return rt_data
    
    def b7n2_sanitize_price(self, raw_str: str) -> Optional[str]:
        """price string parser"""
        if not raw_str:
            return None
        cleaned = re.sub(r'[^\d,.]', '', raw_str)
        cleaned = cleaned.replace(',', '.')
        return cleaned if cleaned else None
    
    async def x1p9_orchestrate(self):
        """main execution flow"""
        async with async_playwright() as pw:
            br = await pw.chromium.launch(
                headless=False,
                args=['--disable-blink-features=AutomationControlled']
            )
            
            ctx = await br.new_context(
                viewport={'width': 1920, 'height': 1080},
                user_agent='Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
            )
            
            await ctx.add_init_script("""
                Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
            """)
            
            pg = await ctx.new_page()
            
            try:
                print("[+] phase_1: credential_injection_sequence")
                await self.n4x1_authenticate(pg)
                print("[✓] auth_state_achieved")
                
                print("[+] phase_2: campaign_navigation_protocol")
                await self.q8w5_kampanya_nav(pg)
                print("[✓] target_section_reached")
                
                print("[+] phase_3: data_extraction_operation")
                harvested = await self.r3d7_harvest_campaign_data(pg)
                print("[✓] extraction_complete")
                
                print("\n" + "="*60)
                print(f"EXTRACTED_ENTITIES: {harvested['f4_metrics'].get('t1_total', 0)}")
                print(f"AVAILABILITY_STATUS: {harvested['f4_metrics'].get('a7_available', 0)}")
                
                if harvested['f4_metrics'].get('d3_avg_discount'):
                    print(f"AVG_REDUCTION_PERCENT: {harvested['f4_metrics']['d3_avg_discount']}%")
                    print(f"MAX_REDUCTION_PERCENT: {harvested['f4_metrics']['d5_max_discount']}%")
                
                print("\n[SAMPLE_ENTITIES]")
                for ent in harvested["z9_entities"][:5]:
                    if ent.get("y6_label"):
                        print(f"  └─ {ent['y6_label'][:50]}")
                        if ent.get("w3_delta"):
                            print(f"     ├─ REDUCTION: {ent['w3_delta']}%")
                        if ent.get("m5_availability"):
                            print(f"     └─ STATUS: AVAILABLE")
                
                print("="*60)
                
                await asyncio.sleep(3)
                
            except Exception as e:
                print(f"[!] CRITICAL_FAILURE: {type(e).__name__}")
                print(f"[!] ERROR_DETAIL: {str(e)}")
                
            finally:
                await ctx.close()
                await br.close()


async def main():
    orchestrator = h7k2m9()
    await orchestrator.x1p9_orchestrate()


if __name__ == "__main__":
    asyncio.run(main())