Pod::Spec.new do |s|
  s.name           = 'TesseraCore'
  s.version        = '1.0.0'
  s.summary        = 'Tessera Go core (gomobile) bound for the mobile app'
  s.description    = 'Embeds the Tessera Go core via gomobile and exposes a JSON command surface.'
  s.author         = 'bm-197'
  s.homepage       = 'https://github.com/bm-197/tessera'
  s.license        = { :type => 'MIT' }
  s.platforms      = { :ios => '16.4' }
  s.swift_version  = '5.9'
  s.source         = { git: '' }
  s.static_framework = true

  s.dependency 'ExpoModulesCore'

  # The gomobile output. Rebuild with:
  #   gomobile bind -target=ios -o mobile/modules/tessera-core/ios/TesseraGo.xcframework \
  #     github.com/bm-197/tessera/core/mobile
  # Named TesseraGo (not Tessera) so the module doesn't clash with the app.
  s.vendored_frameworks = 'TesseraGo.xcframework'

  s.pod_target_xcconfig = {
    'DEFINES_MODULE' => 'YES',
    'SWIFT_COMPILATION_MODE' => 'wholemodule'
  }

  s.source_files = "*.{h,m,mm,swift}"
end
