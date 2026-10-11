import {render,screen,fireEvent,cleanup} from '@testing-library/react';
import {afterEach,it,expect} from 'vitest';
import {SearchField} from './SearchField';
afterEach(cleanup);
it('renders an accessible searchbox and reports value changes',()=>{
  let value='';
  render(<SearchField aria-label="搜索网盘" placeholder="搜索网盘或团队…" value={value} onChange={e=>{value=e.target.value;}}/>);
  const box=screen.getByRole('searchbox',{name:'搜索网盘'});
  expect(box.getAttribute('placeholder')).toBe('搜索网盘或团队…');
  fireEvent.change(box,{target:{value:'团队'}});
  expect(value).toBe('团队');
});
it('keeps an optional hint decorative next to the field',()=>{
  render(<SearchField aria-label="搜索文件" value="" onChange={()=>{}} hint="⌕"/>);
  expect(screen.getByRole('searchbox',{name:'搜索文件'})).toBeTruthy();
  const hint=screen.getByText('⌕');
  expect(hint.tagName).toBe('KBD');
});
