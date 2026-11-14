import assert from 'node:assert/strict';
import {validateEvenHubPageContainer} from '@evenrealities/even_hub_sdk';
import {SETTINGS_MENUS,settingsMenuItems,settingsPickerPage} from './settings-menu.mjs';
for(const menu of settingsMenuItems())assert.ok(new TextEncoder().encode(menu.itemName).length<=32);
assert.ok(settingsMenuItems().length+3<=10);
assert.ok(!settingsMenuItems(false).some(item=>item.itemID===9));
for(const menu of Object.values(SETTINGS_MENUS)){
  const page=settingsPickerPage({...menu,value:menu.options[0][0]});
  const result=validateEvenHubPageContainer(page);
  assert.equal(result.valid,true,JSON.stringify(result));
  assert.equal(page.listObject[0].isEventCapture,1);
  assert.ok(menu.options.length<=20);
  for(const [,label] of menu.options)assert.ok(Array.from(label).length+2<=64);
}
console.log('PASS: existing menu names, SDK menu byte/count limits, native list selection pages and geometry');
