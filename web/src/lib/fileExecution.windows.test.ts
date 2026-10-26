import {describe,it,expect} from 'vitest'
import {canExecuteFile,fileExecutionCommand,fileExecutionDisplayCommand} from './fileExecution'
import type {FileEntry} from '@/types/api'

describe('Windows file execution',()=>{
  it('keeps paths out of PowerShell syntax and uses the selected Windows host',()=>{
    const entry={kind:'file',path:'/C/测试/‘; Write-Output injected; ’.ps1',mode:'-rw-rw-rw-'} as FileEntry
    expect(canExecuteFile(entry,'powershell')).toBe(true)
    const command=fileExecutionCommand(entry,{unavailable:'missing',permission:'permission',interpreter:name=>name,tool:name=>name},'powershell')!
    expect(command).toContain('Set-Location -LiteralPath')
    expect(command).toContain('FromBase64String')
    expect(command).not.toContain('injected')
    expect(command).not.toContain('cd --')
    expect(fileExecutionDisplayCommand(entry,'powershell')).toBe(command)
  })
  it('rejects device, relative and newline paths',()=>{
    for(const path of ['C:/test.ps1','/test.ps1','/C/../evil.ps1','/C/a\nWrite-Output.ps1']){
      expect(canExecuteFile({kind:'file',path,mode:'-rw-r--r--'} as FileEntry,'powershell')).toBe(false)
    }
  })
})
