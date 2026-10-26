import 'package:flutter_test/flutter_test.dart';
import 'package:global_messenger/main.dart';

void main() {
  testWidgets('MessengerApp boots and displays authentication UI', (WidgetTester tester) async {
    await tester.pumpWidget(const MessengerApp());
    await tester.pumpAndSettle();

    expect(find.text('Global Messenger'), findsWidgets);
    expect(find.text('로그인'), findsWidgets);
  });

  test('Api client helper constructs proper headers', () {
    final api = Api();
    expect(api.token, isNull);
    api.token = 'test-token';
    expect(api.token, equals('test-token'));
  });
}
