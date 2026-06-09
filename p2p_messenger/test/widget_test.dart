import 'package:flutter_test/flutter_test.dart';
import 'package:p2p_messenger/main.dart';

void main() {
  testWidgets('App renders', (WidgetTester tester) async {
    await tester.pumpWidget(const P2pApp());
    expect(find.text('Connect'), findsOneWidget);
    expect(find.text('Call'), findsOneWidget);
  });
}
